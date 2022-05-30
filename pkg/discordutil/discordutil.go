// Package discordutil provides general purpose extensuion
// functionalities for discordgo.
package discordutil

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/bwmarrin/snowflake"
)

// GetMessageLink assembles and returns a message link by
// passed msg object and guildID.
func GetMessageLink(msg *discordgo.Message, guildID string) string {
	return fmt.Sprintf("https://discord.com/channels/%s/%s/%s", guildID, msg.ChannelID, msg.ID)
}

// GetDiscordSnowflakeCreationTime returns the time.Time
// of creation of the passed snowflake string.
//
// Returns an error when the passed snowflake string could
// not be parsed to an integer.
func GetDiscordSnowflakeCreationTime(snowflake string) (time.Time, error) {
	sfI, err := strconv.ParseInt(snowflake, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	timestamp := (sfI >> 22) + 1420070400000
	return time.Unix(timestamp/1000, timestamp), nil
}

// IsAdmin returns true if one of the members roles has
// admin (0x8) permissions on the passed guild.
func IsAdmin(g *discordgo.Guild, m *discordgo.Member) bool {
	if m == nil || g == nil {
		return false
	}

	for _, r := range g.Roles {
		if r.Permissions&0x8 != 0 {
			for _, mrID := range m.Roles {
				if r.ID == mrID {
					return true
				}
			}
		}
	}

	return false
}

// DeleteMessageLater tries to delete the passed msg after
// the specified duration.
//
// If the message was already removed, the error will be
// ignored.
func DeleteMessageLater(s *discordgo.Session, msg *discordgo.Message, duration time.Duration) {
	if msg == nil {
		return
	}
	time.AfterFunc(duration, func() {
		s.ChannelMessageDelete(msg.ChannelID, msg.ID)
	})
}

// IsCanNotOpenDmToUserError returns true if an returned error
// is caused because a DM channel to a user could not be opened.
func IsCanNotOpenDmToUserError(err error) bool {
	return err != nil && strings.Contains(err.Error(), `"Cannot send messages to this user"`)
}

// IsErrCode returns true when the given error is a discordgo
// RESTError with the given code.
func IsErrCode(err error, code int) bool {
	apiErr, ok := err.(*discordgo.RESTError)
	return ok && apiErr.Message.Code == code
}

// GetShardOfGuild parses the passed guild ID into a snowflake.
// Then, the ID of the corresponding shard ID is calculated using
// the formula
//   shardId = (guildId >> 22) % numShards
// as documented here
//   https://discord.com/developers/docs/topics/gateway#sharding
func GetShardOfGuild(guildID string, numShards int) (id int, err error) {
	sf, err := snowflake.ParseString(guildID)
	if err != nil {
		return
	}
	id = int((sf.Int64() >> 22) % int64(numShards))
	return
}

// GetShardOfSession returns the current shard ID and
// total shard count set in the passed session.
func GetShardOfSession(s *discordgo.Session) (id, total int) {
	if s.Identify.Shard == nil {
		return
	}
	sh := *s.Identify.Shard
	id = sh[0]
	total = sh[1]
	return
}

// SendDMEmbed sends a DM to the given user ID by opening
// a DM channel to the user and sending the message.
func SendDMEmbed(
	s *discordgo.Session,
	userID string,
	emb *discordgo.MessageEmbed,
) (msg *discordgo.Message, err error) {
	ch, err := s.UserChannelCreate(userID)
	if err != nil {
		return
	}
	msg, err = s.ChannelMessageSendEmbed(ch.ID, emb)
	return
}
