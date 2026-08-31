// audio_test.go — decodificação e registro, headless: nenhum teste cria
// audio.Context (não há dispositivo de som no CI).
package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wavBytes gera um WAV PCM 16-bit mono 8000 Hz com nSamples amostras.
func wavBytes(nSamples int) []byte {
	var b bytes.Buffer
	dataLen := nSamples * 2
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+dataLen))
	b.WriteString("WAVEfmt ")
	binary.Write(&b, binary.LittleEndian, uint32(16))
	binary.Write(&b, binary.LittleEndian, uint16(1)) // PCM
	binary.Write(&b, binary.LittleEndian, uint16(1)) // mono
	binary.Write(&b, binary.LittleEndian, uint32(8000))
	binary.Write(&b, binary.LittleEndian, uint32(8000*2))
	binary.Write(&b, binary.LittleEndian, uint16(2))
	binary.Write(&b, binary.LittleEndian, uint16(16))
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(dataLen))
	for i := 0; i < nSamples; i++ {
		binary.Write(&b, binary.LittleEndian, int16(math.Sin(float64(i)*0.3)*8000))
	}
	return b.Bytes()
}

func writeTempWav(t *testing.T, nSamples int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "s.wav")
	if err := os.WriteFile(path, wavBytes(nSamples), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDecodeAudioWav(t *testing.T) {
	stream, err := decodeAudio(bytes.NewReader(wavBytes(800)))
	if err != nil {
		t.Fatal(err)
	}
	pcm, err := io.ReadAll(stream)
	if err != nil {
		t.Fatal(err)
	}
	// 800 amostras a 8 kHz = 0,1 s → PCM estéreo 16-bit (4 bytes por quadro)
	if len(pcm) == 0 || len(pcm)%4 != 0 {
		t.Fatalf("PCM inesperado: %d bytes", len(pcm))
	}
}

func TestDecodeAudioOggInvalid(t *testing.T) {
	_, err := decodeAudio(bytes.NewReader([]byte("OggS garbage garbage")))
	if err == nil || !strings.Contains(err.Error(), "ogg") {
		t.Fatalf("want ogg error, got %v", err)
	}
}

func TestDecodeAudioGarbage(t *testing.T) {
	_, err := decodeAudio(bytes.NewReader([]byte("not audio at all")))
	if err == nil || !strings.Contains(err.Error(), "wav") {
		t.Fatalf("want wav error, got %v", err)
	}
}

func TestDecodeAudioTooShort(t *testing.T) {
	_, err := decodeAudio(bytes.NewReader([]byte("ab")))
	if err == nil {
		t.Fatal("want error for short input")
	}
}

func TestLoadSoundRegisters(t *testing.T) {
	a := newAudioEngine()
	id1, err := a.loadSound(writeTempWav(t, 400))
	if err != nil {
		t.Fatal(err)
	}
	id2, err := a.loadSound(writeTempWav(t, 400))
	if err != nil {
		t.Fatal(err)
	}
	if id1 != 1 || id2 != 2 {
		t.Fatalf("want ids 1,2, got %d,%d", id1, id2)
	}
	if len(a.sounds[id1]) == 0 {
		t.Fatal("PCM não registrado")
	}
	if a.ctx != nil {
		t.Fatal("loadSound não deve criar o audio.Context")
	}
}

func TestLoadSoundMissingFile(t *testing.T) {
	a := newAudioEngine()
	if _, err := a.loadSound(filepath.Join(t.TempDir(), "nope.wav")); err == nil {
		t.Fatal("want error for missing file")
	}
}

func TestSetVolume(t *testing.T) {
	a := newAudioEngine()
	if err := a.setVolume(0.5); err != nil || a.volume != 0.5 {
		t.Fatalf("got err=%v volume=%g", err, a.volume)
	}
	for _, v := range []float64{-0.1, 1.5} {
		if err := a.setVolume(v); err == nil || !strings.Contains(err.Error(), "volume must be between 0 and 1") {
			t.Fatalf("volume %g: want range error, got %v", v, err)
		}
	}
	if a.ctx != nil {
		t.Fatal("setVolume não deve criar o audio.Context")
	}
}
