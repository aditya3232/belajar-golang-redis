package belajar_golang_redis

import (
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// TOPIC 11: REDIS STREAM FUNDAMENTALS
// ============================================================================
// Fundamental:
// Redis Stream (hadir sejak Redis 5.0) adalah struktur data append-only log mutakhir
// yang dirancang untuk arsitektur Event-Driven dan Message Streaming.
// Sangat mirip dengan konsep inti **Apache Kafka**, namun terintegrasi langsung
// di dalam Redis dengan setup yang sangat ringan!
//
// Anatomi ID Pesan di Stream:
// Setiap pesan di Stream memiliki ID unik berformat: `<timestamp_ms>-<sequence>`
// Contoh: `1698765432100-0` (Waktu dalam milidetik dan sequence counter jika pesan
// tiba di milidetik yang sama persis). Karakter `*` meminta Redis membuat ID otomatis.
//
// 🏆 PERBEDAAN STREAM VS PUB/SUB & LIST:
// 1. Vs Pub/Sub: Pada Pub/Sub jika subscriber offline, pesan hilang (Fire-and-Forget).
//    Pada Stream, pesan tersimpan permanen (persistent) di disk/RAM hingga di-trim.
// 2. Vs List: List tidak memiliki konsep Consumer Group atau Acknowledgement bawaan.
//    Jika worker List crash saat memproses item yang di-pop, pesan tersebut hilang.
//    Pada Stream, pesan tercatat di Pending Entries List (PEL) sampai dikonfirmasi dengan `XACK`.
//
// Fitur Unggulan Consumer Group:
// - Membagi beban kerja ke banyak worker (Load Balancing).
// - Menjamin satu pesan hanya diproses oleh satu consumer di grup yang sama.
// - Mendukung Retry dan Crash Recovery untuk pesan yang belum di-ACK.
//
// Use Cases Populer:
// 1. Audit Log dan Activity Stream (Pencatatan riwayat transaksi pengguna).
// 2. Microservices Event Sourcing (Event OrderCreated, PaymentCompleted).
// 3. Reliable Asynchronous Task Processing.
// ============================================================================

func TestStreamBasicAddAndRead(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	streamKey := UniqueKey("stream:orders")
	CleanKey(t, client, ctx, streamKey)

	// 1. XADD: Memasukkan event baru ke stream
	// Menggunakan ID "*" agar Redis membuat timestamp ID otomatis
	id1, err := client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		ID:     "*",
		Values: map[string]interface{}{
			"order_id": "ORD-101",
			"total":    150000,
			"status":   "CREATED",
		},
	}).Result()
	assert.NoError(t, err)
	assert.NotEmpty(t, id1)

	id2, err := client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		ID:     "*",
		Values: map[string]interface{}{
			"order_id": "ORD-102",
			"total":    275000,
			"status":   "PAID",
		},
	}).Result()
	assert.NoError(t, err)
	assert.NotEmpty(t, id2)

	// 2. XLEN: Menghitung total pesan di dalam stream
	streamLength, err := client.XLen(ctx, streamKey).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(2), streamLength)

	// 3. XRANGE: Membaca riwayat pesan dari ID awal sampai akhir
	// "-" artinya dari pesan paling awal, "+" artinya sampai pesan paling akhir
	messages, err := client.XRange(ctx, streamKey, "-", "+").Result()
	assert.NoError(t, err)
	assert.Len(t, messages, 2)

	// Verifikasi isi pesan pertama
	assert.Equal(t, id1, messages[0].ID)
	assert.Equal(t, "ORD-101", messages[0].Values["order_id"])

	// 4. XREAD: Membaca pesan baru secara streaming (bisa dengan blocking wait)
	// Membaca pesan yang ID-nya lebih besar dari id1
	streams, err := client.XRead(ctx, &redis.XReadArgs{
		Streams: []string{streamKey, id1},
		Count:   10,
	}).Result()
	assert.NoError(t, err)
	assert.Len(t, streams, 1)
	assert.Len(t, streams[0].Messages, 1)
	assert.Equal(t, id2, streams[0].Messages[0].ID)
}

func TestStreamConsumerGroupAndAck(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	streamKey := UniqueKey("stream:payments")
	groupName := "payment_processors"
	CleanKey(t, client, ctx, streamKey)

	// 1. XGROUP CREATE: Membuat consumer group baru
	// Parameter "0": grup akan membaca dari pesan paling awal yang ada di stream
	// MkStream: true -> otomatis membuat stream key jika belum ada di Redis
	err := client.XGroupCreateMkStream(ctx, streamKey, groupName, "0").Err()
	assert.NoError(t, err)

	// 2. Terbitkan 2 pesan baru ke stream
	msgID1, _ := client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		ID:     "*",
		Values: map[string]interface{}{"invoice": "INV-001", "amount": 50000},
	}).Result()

	msgID2, _ := client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		ID:     "*",
		Values: map[string]interface{}{"invoice": "INV-002", "amount": 90000},
	}).Result()

	// 3. XREADGROUP: Consumer-1 membaca pesan yang belum pernah dibaca siapapun
	// ID ">" adalah simbol sakti: artinya "hanya ambil pesan baru yang belum pernah
	// diberikan ke consumer mana pun di grup ini".
	consumer1Read, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    groupName,
		Consumer: "worker-instance-1",
		Streams:  []string{streamKey, ">"},
		Count:    1, // Hanya ambil 1 pesan
		Block:    1 * time.Second,
	}).Result()

	assert.NoError(t, err)
	assert.Len(t, consumer1Read[0].Messages, 1)
	readMsg1 := consumer1Read[0].Messages[0]
	assert.Equal(t, msgID1, readMsg1.ID)
	assert.Equal(t, "INV-001", readMsg1.Values["invoice"])

	// 4. Consumer-2 membaca pesan berikutnya
	consumer2Read, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    groupName,
		Consumer: "worker-instance-2",
		Streams:  []string{streamKey, ">"},
		Count:    1,
	}).Result()

	assert.NoError(t, err)
	assert.Len(t, consumer2Read[0].Messages, 1)
	readMsg2 := consumer2Read[0].Messages[0]
	assert.Equal(t, msgID2, readMsg2.ID)
	assert.Equal(t, "INV-002", readMsg2.Values["invoice"])

	// 5. XPENDING: Memeriksa antrean pesan yang sedang diproses tapi belum di-ACK
	pendingBeforeAck, err := client.XPending(ctx, streamKey, groupName).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(2), pendingBeforeAck.Count, "Dua pesan berstatus pending sebelum konfirmasi XACK")

	// 6. XACK: Worker 1 berhasil memproses dan mengirimkan konfirmasi (Acknowledgement)
	ackedCount, err := client.XAck(ctx, streamKey, groupName, readMsg1.ID).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), ackedCount)

	// Verifikasi sisa pending berkurang menjadi 1
	pendingAfterAck, _ := client.XPending(ctx, streamKey, groupName).Result()
	assert.Equal(t, int64(1), pendingAfterAck.Count)
}
