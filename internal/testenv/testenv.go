// +build !uintonly

package testenv

import (
	"os"
	"sync"
	"testing"

	"github.com/diamondburned/arikawa/v2/discord"
	"github.com/pkg/errors"
)

type Env struct {
	BotToken  string
	ChannelID discord.ChannelID
	VoiceChID discord.ChannelID
}

var (
	env  Env
	err  error
	once sync.Once
)

func Must(t *testing.T) Env {
	e, err := GetEnv()
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func GetEnv() (Env, error) {
	once.Do(getEnv)
	return env, err
}

func getEnv() {
	var token = os.Getenv("BOT_TOKEN")
	if token == "" {
		err = errors.New("missing $BOT_TOKEN")
		return
	}

	var cid = os.Getenv("CHANNEL_ID")
	if cid == "" {
		err = errors.New("missing $CHANNEL_ID")
		return
	}

	id, err := discord.ParseSnowflake(cid)
	if err != nil {
		err = errors.Wrap(err, "invalid $CHANNEL_ID")
		return
	}

	var sid = os.Getenv("VOICE_ID")
	if sid == "" {
		err = errors.New("missing $VOICE_ID")
		return
	}

	vid, err := discord.ParseSnowflake(sid)
	if err != nil {
		err = errors.Wrap(err, "invalid $VOICE_ID")
		return
	}

	env = Env{
		BotToken:  token,
		ChannelID: discord.ChannelID(id),
		VoiceChID: discord.ChannelID(vid),
	}
}
