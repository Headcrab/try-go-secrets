package services

import (
	"bytes"
	"testing"
)

func TestBuildSpeechKitPayload(t *testing.T) {
	payload := buildSpeechKitPayload("hello", speechKitPayloadOptions{
		folderID:        "folder",
		voice:           "alena",
		emotion:         "good",
		speed:           1.1,
		format:          "lpcm",
		lang:            "ru-RU",
		sampleRateHertz: 48000,
	})

	if payload.Get("text") != "hello" {
		t.Fatalf("unexpected text: %q", payload.Get("text"))
	}
	if payload.Get("folderId") != "folder" {
		t.Fatalf("unexpected folderId: %q", payload.Get("folderId"))
	}
	if payload.Get("voice") != "alena" {
		t.Fatalf("unexpected voice: %q", payload.Get("voice"))
	}
	if payload.Get("emotion") != "good" {
		t.Fatalf("unexpected emotion: %q", payload.Get("emotion"))
	}
	if payload.Get("speed") != "1.1" {
		t.Fatalf("unexpected speed: %q", payload.Get("speed"))
	}
	if payload.Get("sampleRateHertz") != "48000" {
		t.Fatalf("unexpected sample rate: %q", payload.Get("sampleRateHertz"))
	}
}

func TestWrapLPCMAsWAV(t *testing.T) {
	raw := []byte{0x00, 0x00, 0xff, 0x7f}

	wav, err := wrapLPCMAsWAV(raw, 16000)
	if err != nil {
		t.Fatalf("wrapLPCMAsWAV returned error: %v", err)
	}
	if len(wav) != 44+len(raw) {
		t.Fatalf("unexpected wav size: %d", len(wav))
	}
	if !bytes.Equal(wav[:4], []byte("RIFF")) {
		t.Fatalf("missing RIFF header")
	}
	if !bytes.Equal(wav[8:12], []byte("WAVE")) {
		t.Fatalf("missing WAVE marker")
	}
}
