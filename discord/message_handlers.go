package discord

import (
	"github.com/denverquane/amongusdiscord/game"
	"log"

	"github.com/bwmarrin/discordgo"
)

func (guild *GuildState) handleGameEndMessage(s *discordgo.Session) {
	guild.AmongUsData.SetAllAlive()
	guild.AmongUsData.SetPhase(game.LOBBY)

	// apply the unmute/deafen to users who have state linked to them
	guild.handleTrackedMembers(s, 0, NoPriority)

	//clear the tracking and make sure all users are unlinked
	guild.clearGameTracking(s)

	// clear any existing game state message
	guild.AmongUsData.SetRoomRegion("", "")
}

func (guild *GuildState) handleGameStartMessage(s *discordgo.Session, m *discordgo.MessageCreate, room string, region string, channels []TrackingChannel) {
	guild.AmongUsData.SetRoomRegion(room, region)

	guild.clearGameTracking(s)

	for _, channel := range channels {
		if channel.channelName != "" {
			guild.Tracking.AddTrackedChannel(channel.channelID, channel.channelName, channel.forGhosts)
		}
	}

	guild.GameStateMsg.CreateMessage(s, gameStateResponse(guild), m.ChannelID)

	log.Println("Added self game state message")

	for _, e := range guild.StatusEmojis[true] {
		guild.GameStateMsg.AddReaction(s, e.FormatForReaction())
	}
	guild.GameStateMsg.AddReaction(s, "❌")
}

func (guildState *GuildState) createPrivateMapMessage(s *discordgo.Session, m *discordgo.MessageCreate, channels []TrackingChannel) {
	mapUsers(s, m, channels, guildState)
	createInitialMessageAndUpdatePrintedUsers(s, guildState)
}

// START - createPrivateMapMessage Helper Methods
func mapUsers(s *discordgo.Session, m *discordgo.MessageCreate, channels []TrackingChannel, guildState *GuildState) {

	var idUsernameMap = populateIdUsernameMap(s, m.GuildID, channels, guildState);

	guildState.PrivateStateMsg.idUsernameMap = idUsernameMap;
}

func populateIdUsernameMap(s *discordgo.Session, guildId string, channels []TrackingChannel, guildState *GuildState) map[string]string{
	var idUsernameMap = make(map[string]string);
	var guild = getGuildById(s, guildId);
	// Iterate through all of the voice channels and find ones that are tracked
	for _, vs := range guild.VoiceStates {

		// Determine if this voice channel is tracked
		if !isVCTracked(vs.ChannelID, channels){
			// If the voice channel we're iterating through isn't tracked, ignore it.
			continue;
		}

		// Get the member/user of the voice state
		var member = getMemberById(s, vs.UserID, vs.GuildID);
		var user = member.User;
		if user == nil {
			log.Print("User is nil for it's own voice channel! Unsure how this has happened")
			continue;
		}

		if guildState.UserData.containsUser(user.ID) {
			log.Println("UserData contains user: " + user.ID + " already! skipping...")
			continue;
		} else {
			log.Println("UserData doesn't contain user: " + user.ID + " already! handling...")
		}

		var username = getUsername(member);

		var userID = user.ID
		var targetUsername = idUsernameMap[userID];

		if targetUsername == "" {
			log.Print("UserID hasn't been saved before: USERID:" + userID + " NAME:" + username);
			// user hasn't been saved before.

			//TODO: CHANGE IMPLENTATION TO A MORE OPTIMIZED SOLUTION.
			//PERHAPS USE A CUSTOM OBJECT FOR THE VALUE TYPE OF THE MAP!
			//TRY TO MAKE NAME INFORMATION ACCESSIBLE EASILY

			var discriminationNecessary = false;

			for _, uName := range idUsernameMap {
				if uName == username {
					log.Print("Found a previous user with the same name as current user: " + username);
					log.Print("Setting discrimination necessary to true");
					//Came across exact same name. Update all with the same name to also include the discriminator for the name
					discriminationNecessary = true
					break;
				}
			}

			idUsernameMap[userID] = username;

			if discriminationNecessary {
				log.Print("Determined that discrimination necessary is true");
				usernameIdCopy := make(map[string]string);

				log.Print("Looking through the map for similar values.");
				for uID, uName := range idUsernameMap {
					var value = uName;
					if uName == username {
						log.Print("Found a similar value...");
						// Is a duplicate/original of the name. Add descriminator
						var targetMember, _ = s.State.Member(guildId, uID)
						value = uName + "#" + targetMember.User.Discriminator;
						log.Print("Updating name to use discriminator: " + targetMember.User.Discriminator);
					}

					usernameIdCopy[uID] = value;
				}
				log.Print("Finished iteration through map for similar values.");

				idUsernameMap = usernameIdCopy;

				log.Print("Updated map");


			}
		} else {
			log.Print("User has been saved before: " + user.Username);
			// User has been saved before
		}
	}

	return idUsernameMap
}

func getGuildById(s *discordgo.Session, guildId string) *discordgo.Guild{
	var g,_ = s.State.Guild(guildId);
	return g;
}

func isVCTracked(voiceChannelID string, channels []TrackingChannel) bool {
	for _, a := range channels {
		if a.channelID == voiceChannelID {
			return true;
		}
	}
	return false;
}

func getMemberById(s *discordgo.Session, userId string, guildId string) *discordgo.Member {
	log.Print("UserId: " + userId);
	var member, err = s.State.Member(guildId, userId)
	if err != nil {
		// If we were unable to get member using local/cache information, grab it from the discord API
		// This fallback is much slower, but guaranteed if the first method doesn't work.
		log.Println("Error: " + err.Error());
		log.Println("Falling back to s.GuildMember implementation");
		member, _ = s.GuildMember(guildId, userId);
	}

	if member == nil {
		// If member is still nil for some reason, show an error message;
		log.Print("Member is nil for given user id: " + userId);
		return nil
	}

	return member
}

func getUsername(member *discordgo.Member) string {
	//Assign username to their nick if they have one, otherwise use their username.
	var username = "";
	if member.Nick == "" {
		// Doesn't have a Nick.
		username = member.User.Username;
	} else {
		// Does have a nickname. Use nickname for reference
		username = member.Nick;
	}
	return username;
}

func createInitialMessageAndUpdatePrintedUsers(s *discordgo.Session, guildState *GuildState) {
	var message *discordgo.Message;
	for uID, uName := range guildState.PrivateStateMsg.idUsernameMap {
		message = guildState.PrivateStateMsg.CreateMessage(s, guildState.PrivateStateMsg.privateMapResponse(uID, uName), guildState.PrivateStateMsg.privateChannelID);
		guildState.PrivateStateMsg.printedUsers = append(guildState.PrivateStateMsg.printedUsers, uID);
		guildState.PrivateStateMsg.currentUserID = uID;
		break;
	}

	//TODO: Instead of returning here, determine before the handleCreatePrivateState if the private channel id is correct.
	if message == nil {
		log.Print("Message nil, skipping create message");
		return;
	}

	guildState.PrivateStateMsg.message = message;

	for _, e := range guildState.StatusEmojis[true] {
		guildState.PrivateStateMsg.AddReaction(s, e.FormatForReaction())
	}
	guildState.PrivateStateMsg.AddReaction(s, "❌")
}

// END - createPrivateMapMessage Helper Methods

// sendMessage provides a single interface to send a message to a channel via discord
func sendMessage(s *discordgo.Session, channelID string, message string) *discordgo.Message {
	msg, err := s.ChannelMessageSend(channelID, message)
	if err != nil {
		log.Println(err)
	}
	return msg
}

func sendMessageEmbed(s *discordgo.Session, channelID string, message *discordgo.MessageEmbed) *discordgo.Message {
	msg, err := s.ChannelMessageSendEmbed(channelID, message)
	if err != nil {
		log.Println(err)
	}
	return msg
}

// editMessage provides a single interface to edit a message in a channel via discord
func editMessage(s *discordgo.Session, channelID string, messageID string, message string) *discordgo.Message {
	msg, err := s.ChannelMessageEdit(channelID, messageID, message)
	if err != nil {
		log.Println(err)
	}
	return msg
}

func editMessageEmbed(s *discordgo.Session, channelID string, messageID string, message *discordgo.MessageEmbed) *discordgo.Message {
	msg, err := s.ChannelMessageEditEmbed(channelID, messageID, message)
	if err != nil {
		log.Println(err)
	}
	return msg
}

func deleteMessage(s *discordgo.Session, channelID string, messageID string) {
	err := s.ChannelMessageDelete(channelID, messageID)
	if err != nil {
		log.Println(err)
	}
}

func addReaction(s *discordgo.Session, channelID, messageID, emojiID string) {
	err := s.MessageReactionAdd(channelID, messageID, emojiID)
	if err != nil {
		log.Println(err)
	}
}

func removeAllReactions(s *discordgo.Session, channelID, messageID string) {
	err := s.MessageReactionsRemoveAll(channelID, messageID)
	if err != nil {
		log.Println(err)
	}
}
