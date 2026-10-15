package broker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/automuteus/automuteus/v8/pkg/capture"
	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/automuteus/automuteus/v8/pkg/notice"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/go-redis/redis/v8"
	socketio "github.com/googollee/go-socket.io"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const ConnectCodeLength = capture.ConnectCodeLength

// CaptureReadyTTL bounds how long the bot will believe a capture client can apply mutes after the client goes quiet.
// It is refreshed on every event the client sends and cleared on disconnect.
const CaptureReadyTTL = time.Minute * 15

// CaptureHolderTTL only garbage-collects the holder of a connect code nobody uses anymore; a missing holder is read
// as "nobody else holds it", so expiry never retires a capture client.
const CaptureHolderTTL = time.Hour * 24

// ReplacedCloseDelay is how long a replaced capture client gets to act on "killself" before Galactus closes its
// socket. Closing first would race the message and look like a network drop, which capture clients reconnect after,
// replacing the newcomer in turn.
const ReplacedCloseDelay = time.Second * 5

// captureConn is the part of a socket.io connection the broker uses to retire a replaced capture client.
type captureConn interface {
	Emit(msg string, v ...interface{})
	Leave(room string)
	Close() error
}

// captureClient is one connected capture client.
type captureClient struct {
	code string
	conn captureConn
	// kill is closed when the capture client disconnects or is replaced, stopping its workers
	kill chan struct{}
}

type Broker struct {
	client *redis.Client

	// instanceID tells this broker's sockets apart from other replicas', whose socket IDs can collide with ours
	instanceID string

	// map of socket IDs to the capture clients that have sent a connect code
	connections     map[string]*captureClient
	connectionsLock sync.RWMutex

	// DrainDelay is how long Shutdown waits, after failing readiness and refusing new clients, before announcing
	// the shutdown. It gives the orchestrator time to stop routing new capture connections here.
	DrainDelay time.Duration
	draining   atomic.Bool
}

// Draining reports whether the broker has begun shutting down and is refusing new capture clients.
func (broker *Broker) Draining() bool { return broker.draining.Load() }

func NewBroker(redisAddr, redisUser, redisPass string) *Broker {
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Username: redisUser,
		Password: redisPass,
		DB:       0, // use default DB
	})
	var id [8]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return &Broker{
		client:      rdb,
		instanceID:  hex.EncodeToString(id[:]),
		connections: map[string]*captureClient{},
	}
}

// holder is how a socket of this broker is named in rediskey.CaptureHolder.
func (broker *Broker) holder(socketID string) string {
	return broker.instanceID + ":" + socketID
}

// claim makes socket the capture client for connCode, replacing whichever capture client held it before, here or
// on another replica. The holder is written before the socket is registered, so a claim message arriving in between
// can't retire the newcomer. The returned channel is closed once the newcomer disconnects or is replaced in turn.
func (broker *Broker) claim(ctx context.Context, conn captureConn, socketID, connCode string) <-chan struct{} {
	err := broker.client.Set(ctx, rediskey.CaptureHolder(connCode), broker.holder(socketID), CaptureHolderTTL).Err()
	if err != nil {
		log.Println(err)
	}

	broker.connectionsLock.Lock()
	if previous, ok := broker.connections[socketID]; ok {
		// the same socket sent a connect code again; stop the workers of its first one
		close(previous.kill)
	}
	kill := make(chan struct{})
	broker.connections[socketID] = &captureClient{code: connCode, conn: conn, kill: kill}
	broker.connectionsLock.Unlock()

	// the replaced capture client may have been applying mutes; the newcomer must send its own bot ID for that
	if err := broker.client.Del(ctx, rediskey.CaptureMuteReady(connCode)).Err(); err != nil {
		log.Println(err)
	}
	if err := broker.client.Publish(ctx, rediskey.CaptureClaimsChannel, connCode).Err(); err != nil {
		log.Println(err)
	}
	return kill
}

// subscribeClaims listens for connect codes taken over by any replica. It returns once the subscription is active.
func (broker *Broker) subscribeClaims(ctx context.Context) (*redis.PubSub, error) {
	pubsub := broker.client.Subscribe(ctx, rediskey.CaptureClaimsChannel)
	if _, err := pubsub.Receive(ctx); err != nil {
		pubsub.Close()
		return nil, err
	}
	return pubsub, nil
}

// serveClaims retires this broker's capture clients as other capture clients take over their connect codes.
func (broker *Broker) serveClaims(ctx context.Context, pubsub *redis.PubSub) {
	defer pubsub.Close()
	channel := pubsub.Channel()
	for {
		select {
		case msg, ok := <-channel:
			if !ok {
				return
			}
			broker.retireReplaced(ctx, msg.Payload)
		case <-ctx.Done():
			return
		}
	}
}

// retireReplaced tells every capture client of this broker on connCode that isn't its holder to quit, and stops
// listening to it. Its events are ignored from here on, and its disconnect won't unlink the game.
func (broker *Broker) retireReplaced(ctx context.Context, connCode string) {
	// Read the holder under the lock: claim writes the holder before registering its socket, so a socket registered
	// after this read can't be mistaken for one the holder replaced.
	broker.connectionsLock.Lock()
	holder, err := broker.client.Get(ctx, rediskey.CaptureHolder(connCode)).Result()
	if err != nil {
		broker.connectionsLock.Unlock()
		if !errors.Is(err, redis.Nil) {
			log.Println(err)
		}
		return
	}

	var replaced []*captureClient
	for socketID, c := range broker.connections {
		if c.code == connCode && broker.holder(socketID) != holder {
			log.Printf("Capture client %s replaced by %s for connect code %s", broker.holder(socketID), holder, connCode)
			replaced = append(replaced, c)
			delete(broker.connections, socketID)
		}
	}
	broker.connectionsLock.Unlock()

	for _, c := range replaced {
		close(c.kill)
		c.conn.Leave(connCode)
		c.conn.Emit(capture.KillSelfEvent, "replaced")
		conn := c.conn
		// a capture client that obeyed has already gone; closing a socket.io conn twice is a no-op
		time.AfterFunc(ReplacedCloseDelay, func() { conn.Close() })
	}
}

// release forgets a disconnected capture client and, unless it was replaced, tells the bot its game is unlinked. It
// returns the connect code whose room should be cleared, if any. A replaced client's late disconnect must leave its
// successor's game alone, even if this broker never heard the claim that replaced it.
func (broker *Broker) release(ctx context.Context, socketID string) (string, bool) {
	broker.connectionsLock.Lock()
	c, ok := broker.connections[socketID]
	delete(broker.connections, socketID)
	broker.connectionsLock.Unlock()
	if !ok {
		// it never sent a connect code, or was already retired as replaced
		return "", false
	}
	close(c.kill)

	holder, err := broker.client.Get(ctx, rediskey.CaptureHolder(c.code)).Result()
	if err == nil && holder != broker.holder(socketID) {
		log.Printf("Replaced capture client %s disconnected; connect code %s stays with %s", broker.holder(socketID), c.code, holder)
		return "", false
	}
	if err != nil && !errors.Is(err, redis.Nil) {
		log.Println(err)
	}

	if err := task.PushJob(ctx, broker.client, c.code, task.ConnectionJob, "false"); err != nil {
		log.Println(err)
	}
	if err := broker.client.Del(ctx, rediskey.CaptureMuteReady(c.code)).Err(); err != nil {
		log.Println(err)
	}
	return c.code, true
}

// connectCode is the connect code a socket reports for, if it has sent one and still holds it.
func (broker *Broker) connectCode(socketID string) (string, bool) {
	broker.connectionsLock.RLock()
	defer broker.connectionsLock.RUnlock()
	c, ok := broker.connections[socketID]
	if !ok {
		return "", false
	}
	return c.code, true
}

func (broker *Broker) TasksListener(server *socketio.Server, connectCode string, killchan <-chan struct{}) {
	pubsub := broker.client.Subscribe(context.Background(), rediskey.TasksList(connectCode))
	log.Println("Task listener OPEN for " + connectCode)
	defer log.Println("Task listener CLOSE for " + connectCode)
	channel := pubsub.Channel()
	for {
		select {
		case t := <-channel:
			taskObj := task.ModifyTask{}

			err := json.Unmarshal([]byte(t.Payload), &taskObj)
			if err != nil {
				log.Println(err)
				break
			}

			log.Println("Broadcasting " + t.Payload + " to room " + connectCode)
			server.BroadcastToRoom("/", connectCode, "modify", t.Payload)

		case <-killchan:
			pubsub.Close()
			return
		}
	}
}

func (broker *Broker) Start(port string) {
	server, err := socketio.NewServer(nil)
	if err != nil {
		log.Fatal(err)
	}
	claims, err := broker.subscribeClaims(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	go broker.serveClaims(context.Background(), claims)

	server.OnConnect("/", func(s socketio.Conn) error {
		s.SetContext("")
		log.Println("connected:", s.ID())
		return nil
	})
	server.OnEvent("/", capture.ConnectCodeEvent, func(s socketio.Conn, msg string) {
		log.Printf("Received connection code: \"%s\"", msg)

		if broker.Draining() {
			// shutting down; the client will reconnect to a healthy replica
			log.Println("Refusing capture client while draining")
			s.Close()
			return
		}
		if len(msg) != ConnectCodeLength {
			s.Close()
		} else {
			kill := broker.claim(context.Background(), s, s.ID(), msg)

			err := task.PushJob(context.Background(), broker.client, msg, task.ConnectionJob, "true")
			if err != nil {
				log.Println(err)
			}
			go broker.AckWorker(context.Background(), msg, kill)
		}
	})

	// only join the room for the connect code once we ensure that the bot actually connects with a valid discord session
	server.OnEvent("/", capture.BotIDEvent, func(s socketio.Conn, msg int64) {
		log.Printf("Received bot ID: \"%d\"", msg)

		broker.connectionsLock.RLock()
		c, ok := broker.connections[s.ID()]
		broker.connectionsLock.RUnlock()
		if !ok {
			log.Println("Got a Bot ID from a socket without a connect code, or one that was replaced; ignoring it")
			return
		}
		// this socket is now listening for mutes that can be applied via that connect code
		s.Join(c.code)
		err := broker.client.Set(context.Background(), rediskey.CaptureMuteReady(c.code), "1", CaptureReadyTTL).Err()
		if err != nil {
			log.Println(err)
		}
		go broker.TasksListener(server, c.code, c.kill)
	})

	server.OnEvent("/", capture.TaskFailedEvent, func(s socketio.Conn, msg string) {
		log.Printf("Received failure for task ID: \"%s\"", msg)

		broker.client.Publish(context.Background(), rediskey.CompleteTask(msg), "false")
	})

	server.OnEvent("/", capture.TaskCompleteEvent, func(s socketio.Conn, msg string) {
		log.Printf("Received success for task ID: \"%s\"", msg)

		broker.client.Publish(context.Background(), rediskey.CompleteTask(msg), "true")
	})

	server.OnEvent("/", capture.LobbyEvent, func(s socketio.Conn, msg string) {
		log.Println("lobby:", msg)

		// validation
		var lobby game.Lobby
		err := json.Unmarshal([]byte(msg), &lobby)
		if err != nil {
			log.Println(err)
		} else {
			if cCode, ok := broker.connectCode(s.ID()); ok {
				err := task.PushJob(context.Background(), broker.client, cCode, task.LobbyJob, msg)
				if err != nil {
					log.Println(err)
				}
				err = broker.client.Set(context.Background(), rediskey.RoomCodesForConnCode(cCode), lobby.LobbyCode, time.Minute*15).Err()
				if err != nil {
					log.Println(err)
				} else {
					log.Printf("Updated room code %s for connect code %s in Redis", lobby.LobbyCode, cCode)
				}
				broker.refreshCaptureReady(cCode)
			}
		}
	})
	server.OnEvent("/", capture.StateEvent, func(s socketio.Conn, msg string) {
		log.Println("phase received from capture: ", msg)
		_, err := strconv.Atoi(msg)
		if err != nil {
			log.Println(err)
		} else {
			if cCode, ok := broker.connectCode(s.ID()); ok {
				err := task.PushJob(context.Background(), broker.client, cCode, task.StateJob, msg)
				if err != nil {
					log.Println(err)
				}
				err = broker.client.Expire(context.Background(), rediskey.RoomCodesForConnCode(cCode), time.Minute*15).Err()
				if !errors.Is(err, redis.Nil) && err != nil {
					log.Println(err)
				}
				broker.refreshCaptureReady(cCode)
			}
		}
	})
	server.OnEvent("/", capture.PlayerEvent, func(s socketio.Conn, msg string) {
		log.Println("player received from capture: ", msg)

		if cCode, ok := broker.connectCode(s.ID()); ok {
			err := task.PushJob(context.Background(), broker.client, cCode, task.PlayerJob, msg)
			if err != nil {
				log.Println(err)
			}
			err = broker.client.Expire(context.Background(), rediskey.RoomCodesForConnCode(cCode), time.Minute*15).Err()
			if !errors.Is(err, redis.Nil) && err != nil {
				log.Println(err)
			}
			broker.refreshCaptureReady(cCode)
		}
	})
	server.OnEvent("/", capture.GameOverEvent, func(s socketio.Conn, msg string) {
		if cCode, ok := broker.connectCode(s.ID()); ok {
			err := task.PushJob(context.Background(), broker.client, cCode, task.GameOverJob, msg)
			if err != nil {
				log.Println(err)
			}
		}
	})
	server.OnError("/", func(s socketio.Conn, e error) {
		log.Println("meet error:", e)
	})
	server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		log.Println("Client connection closed: ", reason)

		if cCode, ok := broker.release(context.Background(), s.ID()); ok {
			server.ClearRoom("/", cCode)
		}
	})
	go server.Serve()
	defer server.Close()

	router := mux.NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, request *http.Request) {
		w.Write([]byte("I'm Alive!"))
	})
	// readiness fails as soon as shutdown begins, so the orchestrator stops sending new capture clients here
	router.HandleFunc("/ready", func(w http.ResponseWriter, request *http.Request) {
		if broker.Draining() {
			http.Error(w, "draining", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("Ready"))
	})
	router.Handle("/socket.io/", server)
	log.Printf("Message broker is running on port %s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}

// Shutdown tells every bot shard that capture connections are about to be severed, so they end running games and
// unmute everyone, and withdraws the capture-ready flags for this broker's clients. Call it on SIGTERM, before exiting.
func (broker *Broker) Shutdown(ctx context.Context) error {
	// stop taking new clients first, then give the orchestrator a moment to notice before announcing; a client
	// that connected in that window would otherwise get a capture-ready flag that outlives this process
	broker.draining.Store(true)
	if broker.DrainDelay > 0 {
		log.Printf("Draining for %s before announcing shutdown", broker.DrainDelay)
		select {
		case <-time.After(broker.DrainDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	broker.connectionsLock.RLock()
	codes := make([]string, 0, len(broker.connections))
	for _, c := range broker.connections {
		codes = append(codes, c.code)
	}
	broker.connectionsLock.RUnlock()

	for _, code := range codes {
		if err := broker.client.Del(ctx, rediskey.CaptureMuteReady(code)).Err(); err != nil {
			log.Println(err)
		}
	}
	// Only this broker's clients are affected: other replicas keep serving their games, and new games are not
	// blocked. A platform-wide outage should be announced through the admin API instead.
	log.Printf("Announcing shutdown to bots; %d capture clients connected", len(codes))
	return notice.AnnounceShutdown(ctx, broker.client, codes)
}

// refreshCaptureReady extends the capture-ready flag for a connect code, if the client has established it.
func (broker *Broker) refreshCaptureReady(connCode string) {
	err := broker.client.Expire(context.Background(), rediskey.CaptureMuteReady(connCode), CaptureReadyTTL).Err()
	if err != nil && !errors.Is(err, redis.Nil) {
		log.Println(err)
	}
}

// anytime a bot "acks", then push a notification
func (broker *Broker) AckWorker(ctx context.Context, connCode string, killChan <-chan struct{}) {
	pubsub := task.AckSubscribe(ctx, broker.client, connCode)
	channel := pubsub.Channel()
	defer pubsub.Close()

	for {
		select {
		case <-killChan:
			return
		case <-channel:
			err := task.PushJob(ctx, broker.client, connCode, task.ConnectionJob, "true")
			if err != nil {
				log.Println(err)
			}
		}
	}
}
