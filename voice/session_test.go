package voice

import (
	"context"
	"encoding/binary"
	"io"
	"log"
	"os"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/internal/testenv"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/ws"
	"github.com/diamondburned/arikawa/v3/voice/voicegateway"
	"github.com/pkg/errors"
)

func TestIntegration(t *testing.T) {
	config := testenv.Must(t)

	ws.WSDebug = func(v ...interface{}) {
		_, file, line, _ := runtime.Caller(1)
		caller := file + ":" + strconv.Itoa(line)
		log.Println(append([]interface{}{caller}, v...)...)
	}

	s := state.New("Bot " + config.BotToken)
	AddIntents(s)

	func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		if err := s.Open(ctx); err != nil {
			t.Fatal("failed to connect:", err)
		}
	}()

	t.Cleanup(func() { s.Close() })

	// Validate the given voice channel.
	c, err := s.Channel(config.VoiceChID)
	if err != nil {
		t.Fatal("failed to get channel:", err)
	}
	if c.Type != discord.GuildVoice {
		t.Fatal("channel isn't a guild voice channel.")
	}

	log.Println("The voice channel's name is", c.Name)

	testVoice(t, s, c)

	// BUG: Discord doesn't want to send the second VoiceServerUpdateEvent. I
	// have no idea why.

	// testVoice(t, s, c)
}

func testVoice(t *testing.T, s *state.State, c *discord.Channel) {
	v, err := NewSession(s)
	if err != nil {
		t.Fatal("failed to create a new voice session:", err)
	}

	// Grab a timer to benchmark things.
	finish := timer()

	// Add handler to receive speaking update beforehand.
	v.AddHandler(func(e *voicegateway.SpeakingEvent) {
		finish("receiving voice speaking event")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	t.Cleanup(cancel)

	if err := v.JoinChannel(ctx, c.ID, false, false); err != nil {
		t.Fatal("failed to join voice:", err)
	}

	t.Cleanup(func() {
		log.Println("Leaving the voice channel concurrently.")

		raceMe(t, "failed to leave voice channel", func() (interface{}, error) {
			return nil, v.Leave(ctx)
		})
	})

	finish("joining the voice channel")

	// Trigger speaking.
	if err := v.Speaking(ctx, voicegateway.Microphone); err != nil {
		t.Fatal("failed to start speaking:", err)
	}

	finish("sending the speaking command")

	f, err := os.Open("testdata/nico.dca")
	if err != nil {
		t.Fatal("failed to open nico.dca:", err)
	}
	defer f.Close()

	var lenbuf [4]byte

	// Copy the audio?
	for {
		if _, err := io.ReadFull(f, lenbuf[:]); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal("failed to read:", err)
		}

		// Read the integer
		framelen := int64(binary.LittleEndian.Uint32(lenbuf[:]))

		// Copy the frame.
		if _, err := io.CopyN(v, f, framelen); err != nil && err != io.EOF {
			t.Fatal("failed to write:", err)
		}
	}

	finish("copying the audio")
}

// raceMe intentionally calls fn multiple times in goroutines to ensure it's not
// racy.
func raceMe(t *testing.T, wrapErr string, fn func() (interface{}, error)) interface{} {
	const n = 3 // run 3 times
	t.Helper()

	// It is very ironic how this method itself is racy.

	var wgr sync.WaitGroup
	var mut sync.Mutex
	var val interface{}
	var err error

	for i := 0; i < n; i++ {
		wgr.Add(1)
		go func() {
			v, e := fn()

			mut.Lock()
			val = v
			err = e
			mut.Unlock()

			if e != nil {
				log.Println("Potential race test error:", e)
			}

			wgr.Done()
		}()
	}

	wgr.Wait()

	if err != nil {
		t.Fatal("Race test failed:", errors.Wrap(err, wrapErr))
	}

	return val
}

// simple shitty benchmark thing
func timer() func(finished string) {
	var then = time.Now()

	return func(finished string) {
		now := time.Now()
		log.Println("Finished", finished+", took", now.Sub(then))
		then = now
	}
}
