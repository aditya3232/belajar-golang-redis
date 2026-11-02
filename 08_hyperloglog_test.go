package belajar_golang_redis

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// TOPIC 8: REDIS HYPERLOGLOG FUNDAMENTALS
// ============================================================================
// Fundamental:
// HyperLogLog (HLL) adalah struktur data probabilistik yang dirancang khusus
// untuk memperkirakan jumlah elemen unik (kardinalitas) dari kumpulan data masif
// dengan efisiensi memori yang luar biasa ekstrem.
//
// Keajaiban Memori (Memory Miracle):
// - Jika Anda menggunakan Redis Set standar untuk menghitung 100 juta User ID unik,
//   Anda membutuhkan RAM hingga beberapa Gigabyte (karena setiap string disimpan).
// - Dengan HyperLogLog, menghitung 100 juta elemen hanya memakan RAM MAKSIMAL 12 KB!
// - Ya, hanya ~12 Kilobyte tetap (fixed memory), baik datanya ratusan maupun ratusan juta!
//
// Trade-Off (Konsekuensi & Batasan):
// 1. Akurasi: Menggunakan estimasi statistik probabilistik dengan standar error ~0.81%.
// 2. Tidak Bisa Mengambil Elemen: Anda TIDAK BISA melihat kembali data apa saja yang sudah
//    dimasukkan (tidak ada perintah seperti `PFMembers`). Hanya bisa melihat estimasi total unik (`PFCount`).
//
// Asal Nama Awalan "PF":
// Perintah HyperLogLog diawali huruf `PF` sebagai penghormatan kepada Philippe Flajolet,
// penemu algoritma HyperLogLog.
//
// Use Cases Populer:
// 1. Menghitung Unique Daily Active Users (DAU) & Monthly Active Users (MAU).
// 2. Menghitung jumlah IP Pengunjung unik pada sebuah website.
// 3. Menghitung jumlah kata kunci unik pencarian pengguna.
// ============================================================================

func TestHyperLogLogBasic(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	hllKey := UniqueKey("hll:visitors:today")
	CleanKey(t, client, ctx, hllKey)

	// 1. PFADD: Menambahkan elemen ke HyperLogLog
	// Return: 1 jika estimasi kardinalitas berubah, 0 jika tidak.
	added, err := client.PFAdd(ctx, hllKey, "user_1", "user_2", "user_3", "user_1").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), added)

	// 2. PFCOUNT: Mengambil estimasi jumlah elemen unik
	count, err := client.PFCount(ctx, hllKey).Result()
	assert.NoError(t, err)
	// Tiga user unik: user_1, user_2, user_3
	assert.Equal(t, int64(3), count)
}

func TestHyperLogLogLargeScale(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	hllKey := UniqueKey("hll:visitors:simulation")
	CleanKey(t, client, ctx, hllKey)

	// Simulasi: Memasukkan 1000 pengunjung unik
	totalInserted := 1000
	var batch []interface{}

	for i := 1; i <= totalInserted; i++ {
		batch = append(batch, fmt.Sprintf("ip_address_192_168_1_%d", i))
		if len(batch) >= 100 {
			_ = client.PFAdd(ctx, hllKey, batch...).Err()
			batch = nil
		}
	}
	if len(batch) > 0 {
		_ = client.PFAdd(ctx, hllKey, batch...).Err()
	}

	// Hitung estimasi
	estimatedCount, err := client.PFCount(ctx, hllKey).Result()
	assert.NoError(t, err)

	// Verifikasi standar deviasi/error berada dalam batas toleransi wajar (< 2%)
	// Untuk 1000 elemen, estimasi biasanya berkisar antara 980 - 1020
	delta := float64(estimatedCount - int64(totalInserted))
	errorPercentage := (delta / float64(totalInserted)) * 100
	if errorPercentage < 0 {
		errorPercentage = -errorPercentage
	}

	t.Logf("Total Asli: %d, Estimasi HLL: %d, Error Margin: %.2f%%",
		totalInserted, estimatedCount, errorPercentage)

	assert.True(t, errorPercentage < 3.0, "Error persentase harus di bawah 3%")
}

func TestHyperLogLogMerge(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	seninKey := UniqueKey("visitors:senin")
	selasaKey := UniqueKey("visitors:selasa")
	mingguanKey := UniqueKey("visitors:mingguan")
	CleanKey(t, client, ctx, seninKey, selasaKey, mingguanKey)

	// Skenario:
	// Senin: user-A, user-B, user-C (3 user unik)
	_ = client.PFAdd(ctx, seninKey, "user-A", "user-B", "user-C").Err()

	// Selasa: user-B, user-C, user-D, user-E (4 user unik, di mana B dan C berulang dari hari Senin)
	_ = client.PFAdd(ctx, selasaKey, "user-B", "user-C", "user-D", "user-E").Err()

	// Fundamental PFMERGE:
	// Menggabungkan beberapa struktur HyperLogLog ke dalam satu key tujuan.
	// Total unik gabungan Senin & Selasa seharusnya: user-A, user-B, user-C, user-D, user-E = 5 unik.
	status, err := client.PFMerge(ctx, mingguanKey, seninKey, selasaKey).Result()
	assert.NoError(t, err)
	assert.Equal(t, "OK", status)

	mergedCount, err := client.PFCount(ctx, mingguanKey).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(5), mergedCount, "Total gabungan unik Senin dan Selasa harus 5")
}
