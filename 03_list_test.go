package belajar_golang_redis

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// TOPIC 3: REDIS LIST FUNDAMENTALS
// ============================================================================
// Fundamental:
// Redis List adalah kumpulan string yang diurutkan berdasarkan urutan penyisipan
// (insertion order).
// Diimplementasikan secara internal menggunakan struktur data `quicklist`
// (kombinasi doubly-linked list dari ziplist).
//
// Karakteristik Kunci:
// - Operasi di ujung kepala (HEAD) dan ujung ekor (TAIL) bernilai O(1):
//   LPUSH, RPUSH, LPOP, RPOP semuanya instan bahkan pada list dengan jutaan elemen!
// - Akses elemen di tengah (via index LINDEX atau LINSERT) bernilai O(N).
//
// Use Cases Populer:
// 1. Message Queue / Job Queue (FIFO - First In First Out)
// 2. Activity Feed / Timeline pengguna (LPUSH + LTRIM untuk menyimpan N aktivitas terakhir)
// 3. Worker Antrean Blocking (BRPOP / BLPOP) tanpa beban polling CPU
// 4. Struktur data Stack (LIFO - Last In First Out)
// ============================================================================

func TestListQueueFIFO(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	queueKey := UniqueKey("queue:tasks")
	CleanKey(t, client, ctx, queueKey)

	// Fundamental Queue (FIFO - First In First Out):
	// Produser memasukkan data dari ujung KANAN (RPush),
	// Konsumen mengambil data dari ujung KIRI (LPop).

	// 1. Masukkan 3 tugas secara berurutan
	err := client.RPush(ctx, queueKey, "task-1", "task-2", "task-3").Err()
	assert.NoError(t, err)

	// Verifikasi panjang antrean dengan LLen
	length, err := client.LLen(ctx, queueKey).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(3), length)

	// 2. Ambil tugas pertama yang masuk (FIFO)
	first, err := client.LPop(ctx, queueKey).Result()
	assert.NoError(t, err)
	assert.Equal(t, "task-1", first, "Item yang pertama masuk harus pertama keluar")

	// 3. Ambil tugas kedua
	second, err := client.LPop(ctx, queueKey).Result()
	assert.NoError(t, err)
	assert.Equal(t, "task-2", second)

	// 4. Ambil tugas ketiga
	third, err := client.LPop(ctx, queueKey).Result()
	assert.NoError(t, err)
	assert.Equal(t, "task-3", third)
}

func TestListStackLIFO(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	stackKey := UniqueKey("stack:undo")
	CleanKey(t, client, ctx, stackKey)

	// Fundamental Stack (LIFO - Last In First Out):
	// Masuk dari KANAN (RPush), keluar dari KANAN (RPop).
	// Contoh: Fitur Undo-Redo di editor teks atau riwayat navigasi balik.

	_ = client.RPush(ctx, stackKey, "action:create", "action:edit_title", "action:change_color").Err()

	// Yang terakhir dimasukkan ("action:change_color") harus yang pertama di-pop
	lastAction, err := client.RPop(ctx, stackKey).Result()
	assert.NoError(t, err)
	assert.Equal(t, "action:change_color", lastAction)
}

func TestListLRangeAndPagination(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	feedKey := UniqueKey("feed:user:101")
	CleanKey(t, client, ctx, feedKey)

	// Tambahkan 5 postingan dari kiri (LPush) sehingga post terbaru selalu ada di index 0
	_ = client.LPush(ctx, feedKey, "post-1", "post-2", "post-3", "post-4", "post-5").Err()

	// Fundamental LRANGE:
	// Format: LRANGE key start stop
	// Index 0 adalah elemen pertama.
	// Index -1 merujuk ke elemen paling terakhir di list.
	// Mengambil seluruh isi list: 0 sampai -1
	allPosts, err := client.LRange(ctx, feedKey, 0, -1).Result()
	assert.NoError(t, err)
	assert.Equal(t, []string{"post-5", "post-4", "post-3", "post-2", "post-1"}, allPosts)

	// Paginasi: Ambil 2 post teratas saja (index 0 sampai 1)
	page1, err := client.LRange(ctx, feedKey, 0, 1).Result()
	assert.NoError(t, err)
	assert.Equal(t, []string{"post-5", "post-4"}, page1)
}

func TestListLTrimCappedList(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	logKey := UniqueKey("logs:recent")
	CleanKey(t, client, ctx, logKey)

	// Fundamental LTRIM (Capped List / Circular Buffer):
	// Memangkas list agar ukurannya tidak membesar tanpa batas hingga memenuhi RAM.
	// Sangat sering digunakan untuk menyimpan misal: "Maksimal 3 log terakhir".

	for i := 1; i <= 5; i++ {
		// Masukkan log baru ke head (kiri)
		_ = client.LPush(ctx, logKey, i).Err()
		// Pangkas hanya menyimpan 3 elemen teratas (index 0 sampai 2)
		_ = client.LTrim(ctx, logKey, 0, 2).Err()
	}

	// List hanya tersisa 3 item terakhir: 5, 4, 3
	items, err := client.LRange(ctx, logKey, 0, -1).Result()
	assert.NoError(t, err)
	assert.Equal(t, []string{"5", "4", "3"}, items)
	assert.Len(t, items, 3)
}

func TestListBlockingPop(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	blockingQueue := UniqueKey("queue:blocking")
	CleanKey(t, client, ctx, blockingQueue)

	// Fundamental BRPOP (Blocking Right Pop):
	// Masalah umum saat membangun worker queue adalah polling terus-menerus (while true { LPop })
	// yang membuang siklus CPU dan bandwidth jaringan ketika antrean kosong.
	// BRPOP secara efisien menidurkan koneksi hingga ada elemen yang masuk ke list
	// ATAU batas waktu timeout tercapai!

	var wg sync.WaitGroup
	var poppedValue string
	var popErr error

	wg.Add(1)
	// Goroutine Worker: Menunggu item baru datang dengan BRPOP
	go func() {
		defer wg.Done()
		// Buka koneksi dedicated atau gunakan client untuk blocking call
		// Parameter timeout: 3 detik
		result, err := client.BRPop(ctx, 3*time.Second, blockingQueue).Result()
		popErr = err
		if err == nil && len(result) == 2 {
			// result[0] adalah nama key, result[1] adalah value yang diambil
			poppedValue = result[1]
		}
	}()

	// Beri jeda 500ms untuk memastikan worker sudah dalam status blocking
	time.Sleep(500 * time.Millisecond)

	// Goroutine Produser: Memasukkan item baru ke antrean
	err := client.LPush(ctx, blockingQueue, "pesan-penting-dari-produser").Err()
	assert.NoError(t, err)

	wg.Wait()

	assert.NoError(t, popErr)
	assert.Equal(t, "pesan-penting-dari-produser", poppedValue)
}
