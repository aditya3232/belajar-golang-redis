package belajar_golang_redis

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// TOPIC 10: REDIS TRANSACTION FUNDAMENTALS
// ============================================================================
// Fundamental:
// Redis menyediakan fitur transaksi menggunakan perintah `MULTI`, `EXEC`, `DISCARD`, dan `WATCH`.
//
// Karakteristik Transaksi Redis:
// 1. Isolasi & Atomisitas Eksekusi:
//    Semua perintah yang dibungkus di antara MULTI dan EXEC dijamin dieksekusi secara
//    berurutan sebagai satu unit kerja tanpa bisa disela (interleaved) oleh client lain.
// 2. Tidak Ada Rollback (No Rollback on Runtime Error):
//    Berbeda dengan database relasional (SQL ACID) yang membatalkan seluruh operasi jika ada
//    error, di Redis jika salah satu perintah gagal karena tipe data salah saat runtime,
//    perintah lain di dalam blok transaksi TETAP DIEKSEKUSI.
//    (Filosofi Redis: bug tipe data adalah kesalahan kode pengembang, bukan kegagalan sistem).
//
// Dua Cara Transaksi di Go-Redis:
// A. `TxPipeline()`: Membungkus batch perintah dalam MULTI / EXEC secara otomatis.
// B. `client.Watch()`: Optimistic Locking (Check-And-Set / CAS) untuk membaca data,
//    melakukan kalkulasi di aplikasi Go, lalu menyimpan kembali tanpa takut balapan
//    dengan request lain (Race Condition).
//
// Use Cases Populer:
// 1. Transfer saldo rekening antar user (debit user A, kredit user B).
// 2. Pengurangan stok produk saat checkout (mencegah overselling).
// ============================================================================

func TestTxPipelineBasic(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	userA := UniqueKey("account:balance:adit")
	userB := UniqueKey("account:balance:budi")
	CleanKey(t, client, ctx, userA, userB)

	// Inisialisasi saldo awal: Adit punya 100.000, Budi punya 50.000
	_ = client.Set(ctx, userA, 100000, 0).Err()
	_ = client.Set(ctx, userB, 50000, 0).Err()

	// Skenario Transaksi: Transfer 20.000 dari Adit ke Budi
	// Kedua operasi harus berhasil bersamaan (MULTI ... EXEC)
	tx := client.TxPipeline()

	// Kurangi saldo Adit sebesar 20.000
	debitCmd := tx.DecrBy(ctx, userA, 20000)
	// Tambah saldo Budi sebesar 20.000
	creditCmd := tx.IncrBy(ctx, userB, 20000)

	// Eksekusi transaksi
	_, err := tx.Exec(ctx)
	assert.NoError(t, err)

	// Periksa hasil
	assert.Equal(t, int64(80000), debitCmd.Val(), "Saldo Adit harus tersisa 80.000")
	assert.Equal(t, int64(70000), creditCmd.Val(), "Saldo Budi harus menjadi 70.000")
}

func TestOptimisticLockingWithWatch(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	stockKey := UniqueKey("inventory:stock:item_1")
	winnerKey := UniqueKey("inventory:winner:item_1")
	CleanKey(t, client, ctx, stockKey, winnerKey)

	// Skenario: Flash Sale dengan stok awal hanya 1 buah
	_ = client.Set(ctx, stockKey, 1, 0).Err()

	// Fungsi beli produk menggunakan Optimistic Locking (client.Watch)
	// Jika key stock diubah oleh pihak ketiga saat fungsi sedang berjalan,
	// transaksi EXEC akan otomatis dibatalkan dan mengembalikan error redis.TxFailedErr.
	buyProduct := func(customerID string) error {
		maxRetries := 3

		for i := 0; i < maxRetries; i++ {
			err := client.Watch(ctx, func(tx *redis.Tx) error {
				// 1. Baca nilai stok saat ini di dalam transaksi WATCH
				valStr, err := tx.Get(ctx, stockKey).Result()
				if err != nil && err != redis.Nil {
					return err
				}

				var stock int
				fmt.Sscanf(valStr, "%d", &stock)

				if stock <= 0 {
					return fmt.Errorf("stok habis")
				}

				// 2. Beri jeda simulasi proses pembayaran / verifikasi
				time.Sleep(50 * time.Millisecond)

				// 3. Simpan perubahan stok dan catat pembeli menggunakan TxPipelined di dalam Watch
				_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
					pipe.Decr(ctx, stockKey)
					pipe.Set(ctx, winnerKey, customerID, 0)
					return nil
				})
				return err
			}, stockKey)

			if err == redis.TxFailedErr {
				// Terjadi konflik race condition, coba ulang (retry)
				continue
			}
			return err
		}
		return fmt.Errorf("gagal setelah mencapai batas retry")
	}

	// Simulasi 2 pengguna mencoba membeli produk terakhir secara BERSAMAAN (konkuren)
	var wg sync.WaitGroup
	var errUser1, errUser2 error

	wg.Add(2)
	go func() {
		defer wg.Done()
		errUser1 = buyProduct("Customer-1")
	}()
	go func() {
		defer wg.Done()
		errUser2 = buyProduct("Customer-2")
	}()

	wg.Wait()

	// Dari 2 pembeli, hanya 1 yang boleh sukses, yang lain harus gagal karena stok habis
	successCount := 0
	if errUser1 == nil {
		successCount++
	}
	if errUser2 == nil {
		successCount++
	}

	assert.Equal(t, 1, successCount, "Hanya tepat 1 pembeli yang berhasil mendapatkan barang!")

	// Sisa stok di Redis harus 0 (tidak boleh minus / overselling)
	finalStock, _ := client.Get(ctx, stockKey).Int()
	assert.Equal(t, 0, finalStock, "Stok akhir harus 0, terlindungi dari race condition")

	// Pemenang tercatat secara atomik di Redis (salah satu dari Customer-1 atau Customer-2)
	winner, err := client.Get(ctx, winnerKey).Result()
	assert.NoError(t, err)
	assert.True(t, winner == "Customer-1" || winner == "Customer-2", "Pemenang harus salah satu customer")
}
