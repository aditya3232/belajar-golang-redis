package belajar_golang_redis

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// TOPIC 4: REDIS SET FUNDAMENTALS
// ============================================================================
// Fundamental:
// Redis Set adalah kumpulan string unik yang TIDAK TERURUT (unordered collection).
// Diimplementasikan secara internal menggunakan Hash Table (atau `intset` jika
// seluruh nilainya integer berukuran kecil demi efisiensi memori).
//
// Karakteristik Kunci:
// - Tidak mengizinkan duplikasi: Jika Anda menambahkan elemen yang sudah ada,
//   Redis mengabaikannya secara otomatis.
// - Pengecekan keanggotaan (SISMEMBER) bernilai O(1) instan!
// - Mendukung operasi aljabar himpunan matematika: IRISAN (Intersection),
//   GABUNGAN (Union), dan SELISIH (Difference) langsung di sisi server.
//
// Use Cases Populer:
// 1. Tagging sistem artikel/produk (e.g. "golang", "redis", "backend").
// 2. Daftar Voter pada polling (mencegah double voting dari satu User ID).
// 3. Rekomendasi pertemanan / Teman bersama (Mutual Friends via SINTER).
// 4. Daftar IP Blacklist atau Unique IP Visitor.
// ============================================================================

func TestSetBasicOperations(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	tagsKey := UniqueKey("article:tags:1")
	CleanKey(t, client, ctx, tagsKey)

	// 1. SADD (Set Add): Menambahkan member ke dalam set.
	// Jika ada duplikasi ("golang" dimasukkan 2x), Redis hanya menyimpannya 1x.
	addedCount, err := client.SAdd(ctx, tagsKey, "golang", "redis", "database", "golang").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(3), addedCount, "Hanya 3 elemen unik yang berhasil ditambahkan")

	// 2. SCARD (Set Cardinality): Menghitung total elemen di dalam set -> O(1)
	cardinality, err := client.SCard(ctx, tagsKey).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(3), cardinality)

	// 3. SISMEMBER: Memeriksa apakah suatu nilai merupakan anggota set -> O(1)
	isMember, err := client.SIsMember(ctx, tagsKey, "golang").Result()
	assert.NoError(t, err)
	assert.True(t, isMember, "'golang' harus ada di dalam set")

	isNotMember, err := client.SIsMember(ctx, tagsKey, "php").Result()
	assert.NoError(t, err)
	assert.False(t, isNotMember, "'php' tidak ada di dalam set")

	// 4. SMEMBERS: Mengambil seluruh isi set -> O(N)
	members, err := client.SMembers(ctx, tagsKey).Result()
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"golang", "redis", "database"}, members)

	// 5. SREM (Set Remove): Menghapus satu atau lebih member dari set
	removedCount, err := client.SRem(ctx, tagsKey, "database").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), removedCount)

	// Verifikasi cardinality berkurang menjadi 2
	cardinalityAfter, _ := client.SCard(ctx, tagsKey).Result()
	assert.Equal(t, int64(2), cardinalityAfter)
}

func TestSetAlgebraOperations(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	// Skenario: Rekomendasi teman / Mutual Friends di media sosial
	userAInterests := UniqueKey("user:interests:adit")
	userBInterests := UniqueKey("user:interests:budi")
	CleanKey(t, client, ctx, userAInterests, userBInterests)

	// Minat Adit: Golang, Redis, Docker, Linux
	_ = client.SAdd(ctx, userAInterests, "golang", "redis", "docker", "linux").Err()

	// Minat Budi: Golang, Redis, Kubernetes, Python
	_ = client.SAdd(ctx, userBInterests, "golang", "redis", "kubernetes", "python").Err()

	// 1. SINTER (Intersection / Irisan):
	// Mencari hobi/minat yang sama-sama dimiliki oleh Adit dan Budi.
	// Hasil yang diharapkan: ["golang", "redis"]
	mutualInterests, err := client.SInter(ctx, userAInterests, userBInterests).Result()
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"golang", "redis"}, mutualInterests)

	// 2. SUNION (Union / Gabungan):
	// Menggabungkan seluruh minat dari Adit dan Budi tanpa duplikasi.
	allInterests, err := client.SUnion(ctx, userAInterests, userBInterests).Result()
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"golang", "redis", "docker", "linux", "kubernetes", "python"}, allInterests)

	// 3. SDIFF (Difference / Selisih):
	// Minat yang dimiliki oleh Adit, tetapi TIDAK dimiliki oleh Budi.
	// userA - userB = ["docker", "linux"]
	aditOnly, err := client.SDiff(ctx, userAInterests, userBInterests).Result()
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"docker", "linux"}, aditOnly)
}

func TestSetRandomAndPop(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	giveawayKey := UniqueKey("giveaway:participants")
	CleanKey(t, client, ctx, giveawayKey)

	// Skenario: Undian Giveaway / Lucky Draw
	_ = client.SAdd(ctx, giveawayKey, "peserta-1", "peserta-2", "peserta-3", "peserta-4", "peserta-5").Err()

	// 1. SRANDMEMBER: Mengambil member acak TANPA menghapusnya dari set
	luckyOne, err := client.SRandMember(ctx, giveawayKey).Result()
	assert.NoError(t, err)
	assert.NotEmpty(t, luckyOne)

	total, _ := client.SCard(ctx, giveawayKey).Result()
	assert.Equal(t, int64(5), total, "SRandMember tidak mengurangi jumlah peserta")

	// 2. SPOP: Mengambil member acak SEKALIGUS MENGHAPUSNYA dari set
	// Sangat cocok untuk undian di mana pemenang tidak boleh diundi lagi
	winner, err := client.SPop(ctx, giveawayKey).Result()
	assert.NoError(t, err)
	assert.NotEmpty(t, winner)

	totalAfterPop, _ := client.SCard(ctx, giveawayKey).Result()
	assert.Equal(t, int64(4), totalAfterPop, "SPop mengurangi 1 pemenang dari set undian")
}
