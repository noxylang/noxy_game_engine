// audio.go — efeitos e música, exports avulsos fora do frame (som não é
// por-frame; ~45 µs por chamada é imperceptível). Efeitos viram PCM na
// memória; música é streamed do arquivo em loop. Só a goroutine do SDK
// toca aqui (concurrency = "single"); o mutex é defensivo. O audio.Context
// nasce preguiçosamente na primeira reprodução — nada aqui exige game_init.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
)

const sampleRate = 48000

type audioEngine struct {
	mu     sync.Mutex
	ctx    *audio.Context   // criado na primeira reprodução (singleton do processo)
	sounds map[int64][]byte // PCM 16-bit estéreo 48 kHz, decodificado inteiro
	nextID int64
	music  *musicEntry
	volume float64 // global 0..1
}

type musicEntry struct {
	player *audio.Player
	file   *os.File
}

func newAudioEngine() *audioEngine {
	return &audioEngine{sounds: map[int64][]byte{}, volume: 1}
}

// decodeAudio detecta o formato pelo conteúdo: "OggS" → vorbis, senão WAV.
func decodeAudio(r io.ReadSeeker) (io.ReadSeeker, error) {
	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return nil, errors.New("not a WAV or OGG file: too short")
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	if bytes.Equal(magic[:], []byte("OggS")) {
		s, err := vorbis.DecodeWithSampleRate(sampleRate, r)
		if err != nil {
			return nil, fmt.Errorf("ogg: %w", err)
		}
		return s, nil
	}
	s, err := wav.DecodeWithSampleRate(sampleRate, r)
	if err != nil {
		return nil, fmt.Errorf("wav: %w", err)
	}
	return s, nil
}

// loadSound decodifica o arquivo inteiro para a memória e devolve o id.
func (a *audioEngine) loadSound(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	stream, err := decodeAudio(f)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", path, err)
	}
	pcm, err := io.ReadAll(stream)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", path, err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.nextID++
	a.sounds[a.nextID] = pcm
	return a.nextID, nil
}

// setVolume aplica na música imediatamente; sons já disparados terminam no
// volume em que começaram.
func (a *audioEngine) setVolume(v float64) error {
	if v < 0 || v > 1 {
		return fmt.Errorf("volume must be between 0 and 1, got %g", v)
	}
	a.mu.Lock()
	a.volume = v
	m := a.music
	a.mu.Unlock()
	if m != nil {
		m.player.SetVolume(v)
	}
	return nil
}

// ensureCtx cria o audio.Context na primeira reprodução (é um singleton do
// processo). Chamar só com a entrada já validada.
func (a *audioEngine) ensureCtx() *audio.Context {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ctx == nil {
		a.ctx = audio.NewContext(sampleRate)
	}
	return a.ctx
}

// playSound toca o som do início, com sobreposição livre (um player novo
// por play, fire-and-forget; o mixer segura o player até o fim).
func (a *audioEngine) playSound(id int64) error {
	a.mu.Lock()
	pcm, ok := a.sounds[id]
	vol := a.volume
	a.mu.Unlock()
	if !ok {
		return fmt.Errorf("unknown sound %d (not returned by load_sound)", id)
	}
	p := a.ensureCtx().NewPlayerFromBytes(pcm)
	p.SetVolume(vol)
	p.Play()
	return nil
}

// playMusic toca o arquivo em streaming, em loop infinito; com música já
// tocando, troca (para e fecha a anterior).
func (a *audioEngine) playMusic(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	stream, err := decodeAudio(f)
	if err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", path, err)
	}
	length, err := stream.Seek(0, io.SeekEnd)
	if err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", path, err)
	}
	if _, err := stream.Seek(0, io.SeekStart); err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", path, err)
	}
	p, err := a.ensureCtx().NewPlayer(audio.NewInfiniteLoop(stream, length))
	if err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", path, err)
	}
	a.mu.Lock()
	old := a.music
	a.music = &musicEntry{player: p, file: f}
	vol := a.volume
	a.mu.Unlock()
	if old != nil {
		old.player.Close()
		old.file.Close()
	}
	p.SetVolume(vol)
	p.Play()
	return nil
}

// stopMusic para e fecha a música. Idempotente.
func (a *audioEngine) stopMusic() {
	a.mu.Lock()
	m := a.music
	a.music = nil
	a.mu.Unlock()
	if m != nil {
		m.player.Close()
		m.file.Close()
	}
}

// ---- handlers (goroutine do SDK) ----

func (a *audioEngine) handleLoadSound(ctx context.Context, path string) (map[string]any, error) {
	id, err := a.loadSound(path)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id}, nil
}

func (a *audioEngine) handlePlaySound(ctx context.Context, id int64) (any, error) {
	return nil, a.playSound(id)
}

func (a *audioEngine) handlePlayMusic(ctx context.Context, path string) (any, error) {
	return nil, a.playMusic(path)
}

func (a *audioEngine) handleStopMusic(ctx context.Context) (any, error) {
	a.stopMusic()
	return nil, nil
}

func (a *audioEngine) handleSetVolume(ctx context.Context, v float64) (any, error) {
	return nil, a.setVolume(v)
}
