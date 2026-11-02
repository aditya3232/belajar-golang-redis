package belajar_golang_redis

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// TOPIC 12: REDIS PUB/SUB (PUBLISH & SUBSCRIBE) FUNDAMENTALS
// ============================================================================
// Fundamental:
// Redis Pub/Sub mengimplementasikan pola pesan Publish/Subscribe yang memisahkan
// pengirim pesan (Publisher) dan penerima pesan (Subscriber) melalui Kanal (Channels).
//
// Karakteristik Kunci:
// 1. Fire-and-Forget (Pesan Efemeral / Tidak Tersimpan):
//    Redis TIDAK MENYIMPAN pesan Pub/Sub di memori atau disk.
//    Jika saat pesan diterbitkan TIDAK ADA subscriber yang aktif mendengarkan,
//    pesan tersebut langsung HILANG selamanya.
//    (Jika Anda butuh pesan yang tersimpan saat worker offline, gunakan REDIS STREAM).
// 2. Sangat Ringan dan Cepat (Sub-millisecond Latency):
//    Karena tidak menulis ke disk atau struktur data persistent, Redis Pub/Sub sangat
//    ideal untuk event berfrekuensi tinggi yang bersifat real-time.
// 3. Pattern Matching (PSubscribe):
//    Subscriber bisa berlangganan ke banyak channel menggunakan pola wildcard
//    (misal: `chat.room.*` atau `notifications.*`).
//
// Use Cases Populer:
// 1. Aplikasi Real-time Chat / Chat Room (penyebaran pesan antar-server WebSocket).
// 2. Cache Invalidation Broadcast (Memberitahu semua node backend untuk membersihkan local cache).
// 3. Notifikasi sistem real-time kepada user yang sedang online.
// ============================================================================

func TestPubSubBasicMessaging(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	channelName := UniqueKey("channel:notifications")

	// 1. Inisialisasi Subscriber pada channel tertentu
	pubsub := client.Subscribe(ctx, channelName)
	defer pubsub.Close()

	// Tunggu sebentar untuk memastikan subscription sudah terdaftar di server Redis
	// (Mencegah pesan terkirim sebelum subscriber benar-benar siap mendengarkan)
	_, err := pubsub.Receive(ctx)
	assert.NoError(t, err)

	var wg sync.WaitGroup
	var receivedPayload string

	wg.Add(1)
	// Goroutine Subscriber: Mendengarkan pesan yang masuk melalui Go channel idiomatik
	go func() {
		defer wg.Done()
		ch := pubsub.Channel()

		select {
		case msg := <-ch:
			receivedPayload = msg.Payload
		case <-time.After(3 * time.Second):
			t.Log("Timeout menunggu pesan dari subscriber")
		}
	}()

	// 2. Goroutine Publisher: Menerbitkan pesan ke channel
	// Perintah PUBLISH mengembalikan jumlah subscriber yang menerima pesan ini
	receiversCount, err := client.Publish(ctx, channelName, "Halo dari Publisher!").Result()
	assert.NoError(t, err)
	assert.True(t, receiversCount >= 1, "Pesan harus diterima oleh minimal 1 subscriber")

	// Tunggu goroutine subscriber selesai membaca
	wg.Wait()

	assert.Equal(t, "Halo dari Publisher!", receivedPayload)
}

func TestPubSubPatternMatching(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	pattern := "sensor.*.temperature"
	channel1 := "sensor.room_101.temperature"
	channel2 := "sensor.room_102.temperature"

	// 1. PSUBSCRIBE: Berlangganan menggunakan pola wildcard "*"
	// Akan menangkap pesan dari sensor suhu di ruangan manapun
	pubsub := client.PSubscribe(ctx, pattern)
	defer pubsub.Close()

	_, err := pubsub.Receive(ctx)
	assert.NoError(t, err)

	var wg sync.WaitGroup
	var receivedMessages []string
	var mu sync.Mutex

	wg.Add(2)
	go func() {
		ch := pubsub.Channel()
		for i := 0; i < 2; i++ {
			select {
			case msg := <-ch:
				mu.Lock()
				receivedMessages = append(receivedMessages, msg.Payload)
				mu.Unlock()
				wg.Done()
			case <-time.After(3 * time.Second):
				wg.Done()
			}
		}
	}()

	// Beri jeda singkat
	time.Sleep(100 * time.Millisecond)

	// 2. Terbitkan pesan dari 2 sensor berbeda yang sama-sama cocok dengan pattern
	_ = client.Publish(ctx, channel1, "Suhu Ruangan 101: 24.5 C").Err()
	_ = client.Publish(ctx, channel2, "Suhu Ruangan 102: 26.0 C").Err()

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, receivedMessages, 2, "Subscriber dengan pattern harus menangkap kedua pesan")
	assert.Contains(t, receivedMessages, "Suhu Ruangan 101: 24.5 C")
	assert.Contains(t, receivedMessages, "Suhu Ruangan 102: 26.0 C")
}
