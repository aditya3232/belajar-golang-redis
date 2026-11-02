package belajar_golang_redis

import (
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// TOPIC 5: REDIS SORTED SET (ZSET) FUNDAMENTALS
// ============================================================================
// Fundamental:
// Redis Sorted Set adalah gabungan luar biasa antara Set dan Hash.
// Setiap anggota di ZSet bersifat UNIK, tetapi diasosiasikan dengan nilai `score`
// berupa angka desimal (`float64`).
//
// Struktur Data Internal:
// Redis menggabungkan dua struktur data sekaligus:
// 1. Hash Table: Memetakan member -> score secara instan O(1).
// 2. Skip List (Daftar Lewat): Struktur data probabilistik mirip pohon biner seimbang
//    yang menjaga elemen tetap terurut berdasarkan score dengan kompleksitas O(log N).
//
// Karakteristik Kunci:
// - Data selalu terurut otomatis setiap ada elemen baru dimasukkan (ZADD).
// - Jika skor dua member sama, Redis mengurutkannya secara leksikografis (abjad).
// - Kompleksitas penambahan dan pencarian ranking: O(log N).
//
// Use Cases Populer:
// 1. Leaderboard Game online (Top 10 Global Players).
// 2. Priority Queue (antrean dengan prioritas berdasarkan bobot / deadline).
// 3. Sliding Window Rate Limiter (menggunakan timestamp sebagai score).
// ============================================================================

func TestSortedSetLeaderboard(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	leaderboardKey := UniqueKey("game:leaderboard")
	CleanKey(t, client, ctx, leaderboardKey)

	// 1. ZADD: Menambahkan pemain beserta skor awal mereka
	// Memberi skor:
	// - Adit: 850.5
	// - Budi: 920.0
	// - Citra: 780.0
	// - Dian: 950.0
	err := client.ZAdd(ctx, leaderboardKey,
		redis.Z{Score: 850.5, Member: "Adit"},
		redis.Z{Score: 920.0, Member: "Budi"},
		redis.Z{Score: 780.0, Member: "Citra"},
		redis.Z{Score: 950.0, Member: "Dian"},
	).Err()
	assert.NoError(t, err)

	// 2. ZCARD: Menghitung jumlah pemain di papan skor
	totalPlayers, err := client.ZCard(ctx, leaderboardKey).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(4), totalPlayers)

	// 3. ZSCORE: Melihat skor pemain tertentu -> O(1)
	aditScore, err := client.ZScore(ctx, leaderboardKey, "Adit").Result()
	assert.NoError(t, err)
	assert.Equal(t, 850.5, aditScore)

	// 4. ZINCRBY: Pemain "Citra" menyelesaikan misi dan mendapat bonus +200 poin
	newCitraScore, err := client.ZIncrBy(ctx, leaderboardKey, 200, "Citra").Result()
	assert.NoError(t, err)
	assert.Equal(t, 980.0, newCitraScore, "780 + 200 = 980, Citra sekarang memimpin!")

	// 5. ZREVRANK: Mengetahui posisi peringkat pemain (0-based) dari skor TERTINGGI ke TERENDAH
	// Urutan skor saat ini:
	// Peringkat 0: Citra (980)
	// Peringkat 1: Dian  (950)
	// Peringkat 2: Budi  (920)
	// Peringkat 3: Adit  (850.5)
	citraRank, err := client.ZRevRank(ctx, leaderboardKey, "Citra").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(0), citraRank, "Citra adalah juara 1 (index 0)")

	budiRank, err := client.ZRevRank(ctx, leaderboardKey, "Budi").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(2), budiRank, "Budi berada di peringkat ke-3 (index 2)")

	// 6. ZREVRANGE: Mengambil TOP 3 Pemain Global (Peringkat 0 sampai 2)
	top3, err := client.ZRevRange(ctx, leaderboardKey, 0, 2).Result()
	assert.NoError(t, err)
	assert.Equal(t, []string{"Citra", "Dian", "Budi"}, top3)

	// 7. ZREVRANGEWITHSCORES: Mengambil data pemain sekaligus skornya
	top3WithScores, err := client.ZRevRangeWithScores(ctx, leaderboardKey, 0, 2).Result()
	assert.NoError(t, err)
	assert.Len(t, top3WithScores, 3)

	assert.Equal(t, "Citra", top3WithScores[0].Member)
	assert.Equal(t, 980.0, top3WithScores[0].Score)

	assert.Equal(t, "Dian", top3WithScores[1].Member)
	assert.Equal(t, 950.0, top3WithScores[1].Score)
}

func TestSortedSetRangeByScore(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	priceRangeKey := UniqueKey("products:price")
	CleanKey(t, client, ctx, priceRangeKey)

	// Skenario: Filter produk berdasarkan rentang harga (Filter e-commerce)
	_ = client.ZAdd(ctx, priceRangeKey,
		redis.Z{Score: 15000, Member: "Buku Tulis"},
		redis.Z{Score: 25000, Member: "Pulpen Gel"},
		redis.Z{Score: 50000, Member: "Tempat Pensil"},
		redis.Z{Score: 120000, Member: "Tas Ransel"},
		redis.Z{Score: 300000, Member: "Sepatu Sekolah"},
	).Err()

	// Fundamental ZRANGEBYSCORE:
	// Mencari semua produk dengan harga antara 20.000 sampai 150.000
	products, err := client.ZRangeByScore(ctx, priceRangeKey, &redis.ZRangeBy{
		Min: "20000",
		Max: "150000",
	}).Result()

	assert.NoError(t, err)
	assert.Equal(t, []string{"Pulpen Gel", "Tempat Pensil", "Tas Ransel"}, products)

	// ZCOUNT: Menghitung berapa banyak item yang memenuhi kriteria filter
	count, err := client.ZCount(ctx, priceRangeKey, "20000", "150000").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)
}
