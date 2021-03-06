package main

import (
    "regexp"
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	//b64 "encoding/base64"
	//"bufio"
	log "github.com/Sirupsen/logrus"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"github.com/layeh/gopus"
	redis "gopkg.in/redis.v3"
)

var (
	// discordgo session
	discord *discordgo.Session

	// Redis client connection (used for stats)
	rcli *redis.Client

	// Map of Guild id's to *Play channels, used for queuing and rate-limiting guilds
	queues map[string]chan *Play = make(map[string]chan *Play)

	// Sound encoding settings
	BITRATE        = 128
	MAX_QUEUE_SIZE = 6

	// Owner
	OWNER string

	// Shard (or -1)
	SHARDS []string = make([]string, 0)
)

// Play represents an individual use of the !airhorn command
type Play struct {
	GuildID   string
	ChannelID string
	UserID    string
	Sound     *Sound

	// The next play to occur after this, only used for chaining sounds like anotha
	Next *Play

	// If true, this was a forced play using a specific airhorn sound name
	Forced bool
}

type SoundCollection struct {
	Prefix    string
	Commands  []string
	Sounds    []*Sound
	ChainWith *SoundCollection

	soundRange int
}

// Sound represents a sound clip
type Sound struct {
	Name string

	// Weight adjust how likely it is this song will play, higher = more likely
	Weight int

	// Delay (in milliseconds) for the bot to wait before sending the disconnect request
	PartDelay int

	// Channel used for the encoder routine
	encodeChan chan []int16

	// Buffer to store encoded PCM packets
	buffer [][]byte
}

// Array of all the sounds we have
var AIRHORN *SoundCollection = &SoundCollection{
	Prefix: "airhorn",
	Commands: []string{
		"!airhorn",
	},
	Sounds: []*Sound{
		createSound("default", 1000, 250),
		createSound("fourtap", 800, 250),
	},
}

var KHALED *SoundCollection = &SoundCollection{
	Prefix:    "another",
	ChainWith: AIRHORN,
	Commands: []string{
		"!anotha",
		"!anothaone",
	},
	Sounds: []*Sound{
		createSound("one", 1, 250),
		createSound("one_classic", 1, 250),
	},
}

var TSM *SoundCollection = &SoundCollection{
	Prefix:    "TSM",
	Commands: []string{
		"!TSM",
		"!bestteam",
	},
	Sounds: []*Sound{
		createSound("TSM", 1, 250),
	},
}

var BELIEVE *SoundCollection = &SoundCollection{
	Prefix:    "TSM",
	Commands: []string{
		"!believe",
		"!cantbelieve",
		"!cb",
	},
	Sounds: []*Sound{
		createSound("cantbelieve", 1, 250),
	},
}

var HOW *SoundCollection = &SoundCollection{
	Prefix:    "TSM",
	Commands: []string{
		"!how",
		"!happentome",
	},
	Sounds: []*Sound{
		createSound("howcouldthis", 1, 250),
	},
}

var FRICK *SoundCollection = &SoundCollection{
	Prefix:    "TSM",
	Commands: []string{
		"!frick",
	},
	Sounds: []*Sound{
		createSound("frick", 1, 250),
	},
}

var WHEN *SoundCollection = &SoundCollection{
	Prefix:    "TSM",
	Commands: []string{
		"!whenwillyoulearn",
		"!wwyl",
	},
	Sounds: []*Sound{
		createSound("whenwilllearn", 1, 250),
	},
}

var EVENNOW *SoundCollection = &SoundCollection{
	Prefix:    "TSM",
	Commands: []string{
		"!evennow",
		"!even",
	},
	Sounds: []*Sound{
		createSound("evennow", 1, 250),
	},
}

var DOIT *SoundCollection = &SoundCollection{
	Prefix:    "TSM",
	Commands: []string{
		"!doit",
		"!justdoit",
	},
	Sounds: []*Sound{
		createSound("justdoit", 1, 250),
	},
}

var OHGOD *SoundCollection = &SoundCollection{
	Prefix:    "TSM",
	Commands: []string{
		"!ohmygod",
		"!omg",
	},
	Sounds: []*Sound{
		createSound("ohgod", 1, 250),
	},
}

var CHERRY *SoundCollection = &SoundCollection{
	Prefix:    "TSM",
	Commands: []string{
		"!rero",
		"!cherry",
	},
	Sounds: []*Sound{
		createSound("cherry", 1, 250),
	},
}

var HELLO *SoundCollection = &SoundCollection{
	Prefix:    "TSM",
	Commands: []string{
		"!hello",
	},
	Sounds: []*Sound{
		createSound("hello", 1, 250),
	},
}

var FUCKEDUP *SoundCollection = &SoundCollection{
	Prefix:    "TSM",
	Commands: []string{
		"!fuckedup",
	},
	Sounds: []*Sound{
		createSound("fuckedup", 1, 250),
	},
}

var ETHAN *SoundCollection = &SoundCollection{
	Prefix: "ethan",
	Commands: []string{
		"!ethan",
		"!eb",
	},
	Sounds: []*Sound{
		createSound("classic", 100, 250),
	},
}

var DOUBLELIFT *SoundCollection = &SoundCollection{
	Prefix: "lol",
	Commands: []string{
		"!doublelift",
		"!dl",
	},
	Sounds: []*Sound{
		createSound("doublelift", 100, 250),
	},
}

var PENTA *SoundCollection = &SoundCollection{
	Prefix: "lol",
	Commands: []string{
		"!penta",
		"!pentakirr",
	},
	Sounds: []*Sound{
		createSound("pentakirr", 1000, 250),
	},
}

var WOW *SoundCollection = &SoundCollection{
	Prefix: "misc",
	Commands: []string{
		"!wow",
	},
	Sounds: []*Sound{
		createSound("wow", 100, 250),
		createSound("waow", 100, 250),
	},
}

var OHBABY *SoundCollection = &SoundCollection{
	Prefix: "misc",
	Commands: []string{
		"!triple",
	},
	Sounds: []*Sound{
		createSound("triple", 100, 250),
	},
}

var NOICE *SoundCollection = &SoundCollection{
	Prefix: "misc",
	Commands: []string{
		"!noice",
		"!nice",
	},
	Sounds: []*Sound{
		createSound("noice", 100, 250),
	},
}

var NEVER *SoundCollection = &SoundCollection{
	Prefix: "misc",
	Commands: []string{
		"!tobi",
		"!never",
	},
	Sounds: []*Sound{
		createSound("never", 100, 250),
	},
}

var CHOCO *SoundCollection = &SoundCollection{
	Prefix: "misc",
	Commands: []string{
		"!chocolate",
		"!choco",
	},
	Sounds: []*Sound{
		createSound("chocolate", 100, 250),
	},
}

var PROFANITY *SoundCollection = &SoundCollection{
	Prefix: "misc",
	Commands: []string{
		"!profanity",
	},
	Sounds: []*Sound{
		createSound("profanity", 100, 250),
	},
}

var CRY *SoundCollection = &SoundCollection{
	Prefix: "misc",
	Commands: []string{
		"!cry",
	},
	Sounds: []*Sound{
		createSound("cry", 100, 250),
	},
}

var LOL *SoundCollection = &SoundCollection{
	Prefix: "misc",
	Commands: []string{
		"!lol",
	},
	Sounds: []*Sound{
		createSound("hot", 100, 250),
	},
}

var ONLYGAME *SoundCollection = &SoundCollection{
	Prefix: "misc",
	Commands: []string{
		"!mad",
		"!game",
	},
	Sounds: []*Sound{
		createSound("onlygame", 100, 250),
	},
}

var SHEEIT *SoundCollection = &SoundCollection{
	Prefix: "misc",
	Commands: []string{
	},
	Sounds: []*Sound{
		createSound("sheeit", 100, 250),
	},
}



var COLLECTIONS []*SoundCollection = []*SoundCollection{
	FUCKEDUP,
	AIRHORN,
	KHALED,
	ETHAN,
	DOUBLELIFT,
	PENTA,
	WOW,
	OHBABY,
	NOICE,
	NEVER,
	CHOCO,
	PROFANITY,
	CRY,
	LOL,
	ONLYGAME,
	SHEEIT,
    TSM,
    OHGOD,
    HELLO,
    CHERRY,
	BELIEVE,
	DOIT,
	HOW,
	FRICK,
	WHEN,
	EVENNOW,
}

// Create a Sound struct
func createSound(Name string, Weight int, PartDelay int) *Sound {
	return &Sound{
		Name:       Name,
		Weight:     Weight,
		PartDelay:  PartDelay,
		encodeChan: make(chan []int16, 10),
		buffer:     make([][]byte, 0),
	}
}

func (sc *SoundCollection) Load() {
	for _, sound := range sc.Sounds {
		sc.soundRange += sound.Weight
		sound.Load(sc)
	}
}

func (s *SoundCollection) Random() *Sound {
	var (
		i      int
		number int = randomRange(0, s.soundRange)
	)

	for _, sound := range s.Sounds {
		i += sound.Weight

		if number < i {
			return sound
		}
	}
	return nil
}

// Encode reads data from ffmpeg and encodes it using gopus
func (s *Sound) Encode() {
	encoder, err := gopus.NewEncoder(48000, 2, gopus.Audio)
	if err != nil {
		fmt.Println("NewEncoder Error:", err)
		return
	}

	encoder.SetBitrate(BITRATE * 1000)
	encoder.SetApplication(gopus.Audio)

	for {
		pcm, ok := <-s.encodeChan
		if !ok {
			// if chan closed, exit
			return
		}

		// try encoding pcm frame with Opus
		opus, err := encoder.Encode(pcm, 960, 960*2*2)
		if err != nil {
			fmt.Println("Encoding Error:", err)
			return
		}

		// Append the PCM frame to our buffer
		s.buffer = append(s.buffer, opus)
	}
}

// Load attempts to load and encode a sound file from disk
func (s *Sound) Load(c *SoundCollection) error {
	s.encodeChan = make(chan []int16, 10)
	defer close(s.encodeChan)
	go s.Encode()

	path := fmt.Sprintf("audio/%v_%v.wav", c.Prefix, s.Name)
	ffmpeg := exec.Command("ffmpeg", "-i", path, "-f", "s16le", "-ar", "48000", "-ac", "2", "pipe:1")

	stdout, err := ffmpeg.StdoutPipe()
	if err != nil {
		fmt.Println("StdoutPipe Error:", err)
		return err
	}

	err = ffmpeg.Start()
	if err != nil {
		fmt.Println("RunStart Error:", err)
		return err
	}

	for {
		// read data from ffmpeg stdout
		InBuf := make([]int16, 960*2)
		err = binary.Read(stdout, binary.LittleEndian, &InBuf)

		// If this is the end of the file, just return
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		}

		if err != nil {
			fmt.Println("error reading from ffmpeg stdout :", err)
			return err
		}

		// write pcm data to the encodeChan
		s.encodeChan <- InBuf
	}
}

// Plays this sound over the specified VoiceConnection
func (s *Sound) Play(vc *discordgo.VoiceConnection) {
	vc.Speaking(true)
	defer vc.Speaking(false)

	for _, buff := range s.buffer {
		vc.OpusSend <- buff
	}
}

// Attempts to find the current users voice channel inside a given guild
func getCurrentVoiceChannel(user *discordgo.User, guild *discordgo.Guild) *discordgo.Channel {
	for _, vs := range guild.VoiceStates {
		if vs.UserID == user.ID {
			channel, _ := discord.State.Channel(vs.ChannelID)
			return channel
		}
	}
	return nil
}

// Whether a guild id is in this shard
func shardContains(guildid string) bool {
	if len(SHARDS) != 0 {
		ok := false
		for _, shard := range SHARDS {
			if len(guildid) >= 5 && string(guildid[len(guildid)-5]) == shard {
				ok = true
				break
			}
		}
		return ok
	}
	return true
}

// Returns a random integer between min and max
func randomRange(min, max int) int {
	rand.Seed(time.Now().UTC().UnixNano())
	return rand.Intn(max-min) + min
}

// Prepares and enqueues a play into the ratelimit/buffer guild queue
func enqueuePlay(user *discordgo.User, guild *discordgo.Guild, coll *SoundCollection, sound *Sound) {
	// Grab the users voice channel
	channel := getCurrentVoiceChannel(user, guild)
	if channel == nil {
		log.WithFields(log.Fields{
			"user":  user.ID,
			"guild": guild.ID,
		}).Warning("Failed to find channel to play sound in")
		return
	}

	// Create the play
	play := &Play{
		GuildID:   guild.ID,
		ChannelID: channel.ID,
		UserID:    user.ID,
		Sound:     sound,
		Forced:    true,
	}

	// If we didn't get passed a manual sound, generate a random one
	if play.Sound == nil {
		play.Sound = coll.Random()
		play.Forced = false
	}

	// If the collection is a chained one, set the next sound
	if coll.ChainWith != nil {
		play.Next = &Play{
			GuildID:   play.GuildID,
			ChannelID: play.ChannelID,
			UserID:    play.UserID,
			Sound:     coll.ChainWith.Random(),
			Forced:    play.Forced,
		}
	}

	// Check if we already have a connection to this guild
	//   yes, this isn't threadsafe, but its "OK" 99% of the time
	_, exists := queues[guild.ID]

	if exists {
		if len(queues[guild.ID]) < MAX_QUEUE_SIZE {
			queues[guild.ID] <- play
		}
	} else {
		queues[guild.ID] = make(chan *Play, MAX_QUEUE_SIZE)
		playSound(play, nil)
	}
}

func trackSoundStats(play *Play) {
	if rcli == nil {
		return
	}

	_, err := rcli.Pipelined(func(pipe *redis.Pipeline) error {
		var baseChar string

		if play.Forced {
			baseChar = "f"
		} else {
			baseChar = "a"
		}

		base := fmt.Sprintf("airhorn:%s", baseChar)
		pipe.Incr("airhorn:total")
		pipe.Incr(fmt.Sprintf("%s:total", base))
		pipe.Incr(fmt.Sprintf("%s:sound:%s", base, play.Sound.Name))
		pipe.Incr(fmt.Sprintf("%s:user:%s:sound:%s", base, play.UserID, play.Sound.Name))
		pipe.Incr(fmt.Sprintf("%s:guild:%s:sound:%s", base, play.GuildID, play.Sound.Name))
		pipe.Incr(fmt.Sprintf("%s:guild:%s:chan:%s:sound:%s", base, play.GuildID, play.ChannelID, play.Sound.Name))
		pipe.SAdd(fmt.Sprintf("%s:users", base), play.UserID)
		pipe.SAdd(fmt.Sprintf("%s:guilds", base), play.GuildID)
		pipe.SAdd(fmt.Sprintf("%s:channels", base), play.ChannelID)
		return nil
	})

	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Warning("Failed to track stats in redis")
	}
}

// Play a sound
func playSound(play *Play, vc *discordgo.VoiceConnection) (err error) {
	log.WithFields(log.Fields{
		"play": play,
	}).Info("Playing sound")

	if vc == nil {
		vc, err = discord.ChannelVoiceJoin(play.GuildID, play.ChannelID, false, false)
		// vc.Receive = false
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("Failed to play sound")
			delete(queues, play.GuildID)
			return err
		}
	}

	// If we need to change channels, do that now
	if vc.ChannelID != play.ChannelID {
		vc.ChangeChannel(play.ChannelID, false, false)
		time.Sleep(time.Millisecond * 125)
	}

	// Track stats for this play in redis
	go trackSoundStats(play)

	// Sleep for a specified amount of time before playing the sound
	time.Sleep(time.Millisecond * 32)
	_ = "breakpoint"
	// Play the sound
	play.Sound.Play(vc)

	// If this is chained, play the chained sound
	if play.Next != nil {
		playSound(play.Next, vc)
	}

	// If there is another song in the queue, recurse and play that
	if len(queues[play.GuildID]) > 0 {
		play := <-queues[play.GuildID]
		playSound(play, vc)
		return nil
	}

	// If the queue is empty, delete it
	time.Sleep(time.Millisecond * time.Duration(play.Sound.PartDelay))
	delete(queues, play.GuildID)
	vc.Disconnect()
	return nil
}

func onReady(s *discordgo.Session, event *discordgo.Ready) {
	log.Info("Recieved READY payload")
	s.UpdateStatus(0, "AAAAAAAAAAA")
}

func onGuildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {
	if !shardContains(event.Guild.ID) {
		return
	}

	if event.Guild.Unavailable != nil {
		return
	}

	for _, channel := range event.Guild.Channels {


		if channel.ID == event.Guild.ID {
			//s.ChannelMessageSend(channel.ID, "**AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA**")
			return
		}
	}
}

func scontains(key string, options ...string) bool {
	for _, item := range options {
		if item == key {
			return true
		}
	}
	return false
}

func calculateAirhornsPerSecond(cid string) {
	current, _ := strconv.Atoi(rcli.Get("airhorn:a:total").Val())
	time.Sleep(time.Second * 10)
	latest, _ := strconv.Atoi(rcli.Get("airhorn:a:total").Val())

	discord.ChannelMessageSend(cid, fmt.Sprintf("Current APS: %v", (float64(latest-current))/10.0))
}

func displayBotStats(cid string) {
	stats := runtime.MemStats{}
	runtime.ReadMemStats(&stats)

	users := 0
	for _, guild := range discord.State.Ready.Guilds {
		users += len(guild.Members)
	}

	w := &tabwriter.Writer{}
	buf := &bytes.Buffer{}

	w.Init(buf, 0, 4, 0, ' ', 0)
	fmt.Fprintf(w, "```\n")
	fmt.Fprintf(w, "Discordgo: \t%s\n", discordgo.VERSION)
	fmt.Fprintf(w, "Go: \t%s\n", runtime.Version())
	fmt.Fprintf(w, "Memory: \t%s / %s (%s total allocated)\n", humanize.Bytes(stats.Alloc), humanize.Bytes(stats.Sys), humanize.Bytes(stats.TotalAlloc))
	fmt.Fprintf(w, "Tasks: \t%d\n", runtime.NumGoroutine())
	fmt.Fprintf(w, "Servers: \t%d\n", len(discord.State.Ready.Guilds))
	fmt.Fprintf(w, "Users: \t%d\n", users)
	fmt.Fprintf(w, "Shards: \t%s\n", strings.Join(SHARDS, ", "))
	fmt.Fprintf(w, "```\n")
	w.Flush()
	discord.ChannelMessageSend(cid, buf.String())
}

// Handles bot operator messages, should be refactored (lmao)
func handleBotControlMessages(s *discordgo.Session, m *discordgo.MessageCreate, parts []string, g *discordgo.Guild) {
	ourShard := shardContains(g.ID)

	if scontains(parts[len(parts)-1], "stats") && ourShard {
		displayBotStats(m.ChannelID)
	} else if scontains(parts[len(parts)-1], "status") {
		guilds := 0
		for _, guild := range s.State.Ready.Guilds {
			if shardContains(guild.ID) {
				guilds += 1
			}
		}
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf(
			"Shard %v contains %v servers",
			strings.Join(SHARDS, ","),
			guilds))
	} else if scontains(parts[len(parts)-1], "aps") && ourShard {
		s.ChannelMessageSend(m.ChannelID, ":ok_hand: give me a sec m8")
		go calculateAirhornsPerSecond(m.ChannelID)
	}
	return
}

func onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if len(m.Content) <= 0 || (m.Content[0] != '!' && len(m.Mentions) != 1) {
		return
	}

	parts := strings.Split(strings.ToLower(m.Content), " ")

	channel, _ := discord.State.Channel(m.ChannelID)
	if channel == nil {
		log.WithFields(log.Fields{
			"channel": m.ChannelID,
			"message": m.ID,
		}).Warning("Failed to grab channel")
		return
	}

	guild, _ := discord.State.Guild(channel.GuildID)
	if guild == nil {
		log.WithFields(log.Fields{
			"guild":   channel.GuildID,
			"channel": channel,
			"message": m.ID,
		}).Warning("Failed to grab guild")
		return
	}

	// If this is a mention, it should come from the owner (otherwise we don't care)
	if len(m.Mentions) > 0 {
		if m.Mentions[0].ID == s.State.Ready.User.ID && m.Author.ID == OWNER && len(parts) > 0 {
			handleBotControlMessages(s, m, parts, guild)
		}
		return
	}

	// If it's not relevant to our shard, just exit
	if !shardContains(guild.ID) {
		return
	}	

	if parts[0] == "!help" || parts[0] == "!commands" || parts[0] == "!h" {
		help := "`List of commands:`\n\n" +
		"`!airhorn !airhorn default !airhorn fourtap !anotha one !anotha one_classic !ethan !dl !penta !wow wow !wow waow !triple !noice !tobi !choco !profanity !cry !lol !game !doit !wwyl !evennow !cantbelieve !rero !omg !fuckedup !game !how `"
		s.ChannelMessageSend(channel.ID, help)
		return
	}

	// Find the collection for the command we got
	for _, coll := range COLLECTIONS {
	    match,_:=regexp.MatchString("she(e+)it", parts[0])
                       
		if scontains(parts[0], coll.Commands...) || (coll.Sounds[0].Name == "SHEEIT" && match == true) {

			// If they passed a specific sound effect, find and select that (otherwise play nothing)
			var sound *Sound
			if len(parts) > 1 {
				for _, s := range coll.Sounds {
					if parts[1] == s.Name {
						sound = s
					}
				}

				if sound == nil {
					return
				}
			}

			go enqueuePlay(m.Author, guild, coll, sound)
			return
		}
	}
}

func main() {
	var (
		Token = flag.String("t", "", "Discord Authentication Token")
		Redis = flag.String("r", "", "Redis Connection String")
		Shard = flag.String("s", "", "Integers to shard by")
		Owner = flag.String("o", "", "Owner ID")
		err   error
	)
	flag.Parse()

	if *Owner != "" {
		OWNER = *Owner
	}

	// Make sure shard is either empty, or an integer
	if *Shard != "" {
		SHARDS = strings.Split(*Shard, ",")

		for _, shard := range SHARDS {
			if _, err := strconv.Atoi(shard); err != nil {
				log.WithFields(log.Fields{
					"shard": shard,
					"error": err,
				}).Fatal("Invalid Shard")
				return
			}
		}
	}

	// Preload all the sounds
	log.Info("Preloading sounds...")
	for _, coll := range COLLECTIONS {
		coll.Load()
	}

	// If we got passed a redis server, try to connect
	if *Redis != "" {
		log.Info("Connecting to redis...")
		rcli = redis.NewClient(&redis.Options{Addr: *Redis, DB: 0})
		_, err = rcli.Ping().Result()

		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Fatal("Failed to connect to redis")
			return
		}
	}

	// Create a discord session
	log.Info("Starting discord session...")
	discord, err = discordgo.New(*Token)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("Failed to create discord session")
		return
	}

	discord.AddHandler(onReady)
	discord.AddHandler(onGuildCreate)
	discord.AddHandler(onMessageCreate)

	err = discord.Open()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("Failed to create discord websocket connection")
		return
	}

	// We're running!
	log.Info("AIRHORNBOT is ready to horn it up.")

	/*
	imgFile, err := os.Open("spongerobot.png")

	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("Image open error")
		return
	}

	defer imgFile.Close()
	fInfo, _ := imgFile.Stat()
	var size int64 = fInfo.Size()
	buf := make([]byte, size)
	fReader := bufio.NewReader(imgFile)
	fReader.Read(buf)

	sEnc := b64.StdEncoding.EncodeToString(buf)
	_,err = discord.UserUpdate("username","password","NOISE BOT", "data:image/png;base64," + sEnc, "")
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("oops")
		return
	}
	*/

	// Wait for a signal to quit
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	<-c
}
