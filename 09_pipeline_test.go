package belajar_golang_redis

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// TOPIC 9: REDIS PIPELINE FUNDAMENTALS
// ============================================================================
// Fundamental:
// Redis adalah server client-server berbasis protokol request/response TCP.
// Pada operasi biasa:
//   Client kirim Cmd 1 -> Tunggu RTT -> Baca Respon 1
//   Client kirim Cmd 2 -> Tunggu RTT -> Baca Respon 2
//   ...
// Jika ada 100 perintah dan latensi jaringan (RTT - Round-Trip Time) adalah 2ms,
// aplikasi Anda menghabiskan 200ms HANYA untuk menunggu paket bolak-balik di kabel!
//
// Cara Kerja Pipelining:
// Client mengumpulkan (buffer) banyak perintah di memory lokal tanpa menunggu respon satu per satu,
// lalu mengirimkannya sekaligus dalam 1 paket TCP ke Redis.
// Redis mengeksekusi semua perintah secara berurutan dan mengembalikan semua respon
// dalam satu aliran paket jaringan.
//
// ⚠️ PERBEDAAN KRUSIAL: PIPELINE vs TRANSACTION (MULTI/EXEC)
// - Pipeline murni optimasi efisiensi jaringan (mengurangi RTT).
// - Perintah di dalam standar Pipeline TIDAK DIJAMIN ATOMIK! Perintah dari client lain
//   masih bisa disisipkan (interleaved) oleh Redis di sela-sela eksekusi batch ini.
// - Jika Anda butuh atomisitas dan isolasi ketat, gunakan `TxPipeline` (MULTI/EXEC).
//
// Use Cases Populer:
// 1. Bulk insertion (memasukkan ribuan data awal / seeding ke cache).
// 2. Batch update counter secara berkala.
// 3. Pengambilan multi-key kompleks dengan tipe data berbeda.
// ============================================================================

func TestPipelineBasic(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	keyPrefix := UniqueKey("pipe:test")
	k1 := fmt.Sprintf("%s:1", keyPrefix)
	k2 := fmt.Sprintf("%s:2", keyPrefix)
	k3 := fmt.Sprintf("%s:3", keyPrefix)
	CleanKey(t, client, ctx, k1, k2, k3)

	// 1. Buat instance Pipeline
	pipe := client.Pipeline()

	// 2. Daftarkan perintah ke dalam pipeline.
	// CATATAN: Pada tahap ini, perintah BELUM dikirim ke server Redis!
	// Objek command yang dikembalikan (`*redis.StatusCmd`, `*redis.StringCmd`)
	// belum memiliki hasil (.Val() masih kosong).
	setCmd1 := pipe.Set(ctx, k1, "Nilai Satu", 10*time.Minute)
	setCmd2 := pipe.Set(ctx, k2, "Nilai Dua", 10*time.Minute)
	incrCmd := pipe.Incr(ctx, k3)
	getCmd1 := pipe.Get(ctx, k1)

	// 3. EXECUTE: Mengirim seluruh batch perintah ke Redis server dalam 1 RTT
	cmds, err := pipe.Exec(ctx)
	assert.NoError(t, err)
	assert.Len(t, cmds, 4, "Harus mengeksekusi tepat 4 perintah")

	// 4. Setelah Exec berhasil, hasil dari masing-masing perintah sudah dapat dibaca
	assert.Equal(t, "OK", setCmd1.Val())
	assert.Equal(t, "OK", setCmd2.Val())
	assert.Equal(t, int64(1), incrCmd.Val())
	assert.Equal(t, "Nilai Satu", getCmd1.Val())
}

func TestPipelineLatencyComparison(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	totalCommands := 50

	// 1. Eksekusi Non-Pipeline (Sequential biasa)
	startSeq := time.Now()
	for i := 0; i < totalCommands; i++ {
		key := fmt.Sprintf("test:seq:%d", i)
		_ = client.Set(ctx, key, i, 1*time.Minute).Err()
		_ = client.Del(ctx, key).Err()
	}
	durationSeq := time.Since(startSeq)

	// 2. Eksekusi Menggunakan Pipeline (Batching)
	startPipe := time.Now()
	pipe := client.Pipeline()
	for i := 0; i < totalCommands; i++ {
		key := fmt.Sprintf("test:pipe:%d", i)
		pipe.Set(ctx, key, i, 1*time.Minute)
		pipe.Del(ctx, key)
	}
	_, err := pipe.Exec(ctx)
	assert.NoError(t, err)
	durationPipe := time.Since(startPipe)

	t.Logf("Waktu Non-Pipeline (%d perintah): %v", totalCommands*2, durationSeq)
	t.Logf("Waktu Pipelining   (%d perintah): %v", totalCommands*2, durationPipe)

	// Pipelining biasanya jauh lebih cepat (seringkali 3x - 10x lipat lebih efisien)
	assert.True(t, durationPipe <= durationSeq, "Pipeline harus lebih cepat atau setara dibanding eksekusi sekuensial")
}
