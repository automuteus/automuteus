package discord

import (
	"github.com/gorilla/mux"
	"github.com/scaleway/scaleway-sdk-go/api/k8s/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"log"
	"net/http"
	"os"
	"strings"
)

func StartHealthCheckServer(port string) {
	r := mux.NewRouter()

	r.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hell yeah"))
	})

	r.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Get("https://discordapp.com/api/v8/gateway")
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		w.WriteHeader(resp.StatusCode)
		if resp.StatusCode == http.StatusOK {
			w.Write([]byte("ready"))
		} else {
			w.Write([]byte("unready"))
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			orgId := os.Getenv("SCW_ORGANIZATION_ID")
			accessKey := os.Getenv("SCW_ACCESS_KEY")
			secretKey := os.Getenv("SCW_SECRET_KEY")
			nodeID := os.Getenv("SCW_NODE_ID")
			if orgId == "" || accessKey == "" || secretKey == "" || nodeID == "" {
				log.Println("One of the Scaleway credentials was null, not replacing any nodes!")
				return
			}

			TerminateScalewayNode(orgId, accessKey, secretKey, nodeID)
		}
	})

	http.ListenAndServe(":"+port, r)
}

func TerminateScalewayNode(org, access, secret, nodeID string) {
	client, err := scw.NewClient(
		// Get your credentials at https://console.scaleway.com/project/credentials
		scw.WithDefaultOrganizationID(org),
		scw.WithAuth(access, secret),
	)
	if err != nil {
		panic(err)
	}

	kApi := k8s.NewAPI(client)

	nodes, err := kApi.ListNodes(&k8s.ListNodesRequest{
		Region:    scw.RegionFrPar,
		ClusterID: os.Getenv("SCW_CLUSTER_ID"),
		PoolID:    nil,
		OrderBy:   "",
		Page:      nil,
		PageSize:  nil,
		Name:      nil,
		Status:    "",
	})
	if err != nil {
		log.Println(err)
		return
	}
	refUid := formatUUID(nodeID)
	log.Println(refUid)
	for _, v := range nodes.Nodes {
		log.Println(v.ID)
		if strings.HasPrefix(v.ID, refUid) {
			nodeID = v.ID
			break
		}
	}

	_, err = kApi.ReplaceNode(&k8s.ReplaceNodeRequest{
		Region: scw.RegionFrPar,
		NodeID: nodeID,
	})
	if err != nil {
		log.Println("Error replacing " + refUid + ": " + err.Error())
	}
}

func formatUUID(id string) string {
	ids := strings.Split(id, "-")
	id = ids[len(ids)-1]
	if len(id) < 20 {
		return ""
	}
	return id[0:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:]
}
