# 🚀 Panduan Pembelajaran Lengkap Golang Redis

Repositori ini adalah panduan komprehensif pembelajaran integrasi **Golang** dengan **Redis** menggunakan driver standar industri [`github.com/redis/go-redis/v9`](https://github.com/redis/go-redis). 

Setiap konsep diimplementasikan dalam bentuk unit test yang dapat dijalankan secara mandiri, lengkap dengan penjelasan teori, cara kerja struktur data internal di memori Redis, analisis kompleksitas waktu ($O$), dan implementasi use case nyata.

---

## 📑 Daftar Isi

1. [Fundamental Arsitektur Redis](#-fundamental-arsitektur-redis)
2. [Prasyarat & Menjalankan Redis](#-prasyarat--menjalankan-redis)
3. [Daftar 12 Modul Pembelajaran](#-daftar-12-modul-pembelajaran)
   - [01. Redis Client & Connection Pooling](#01-redis-client--connection-pooling)
   - [02. String (Caching, TTL, Counter, Distributed Lock)](#02-string)
   - [03. List (Queue FIFO, Stack LIFO, Blocking Pop)](#03-list)
   - [04. Set (Elemen Unik & Aljabar Himpunan)](#04-set)
   - [05. Sorted Set / ZSet (Leaderboard & Peringkat)](#05-sorted-set-zset)
   - [06. Hash (Representasi Objek & Struct)](#06-hash)
   - [07. GeoPoint (Geospasial, Radius & Jarak)](#07-geopoint-geospatial)
   - [08. HyperLogLog (Estimasi Kardinalitas Data Masif)](#08-hyperloglog)
   - [09. Pipeline (Batching & Reduksi Latensi Jaringan / RTT)](#09-pipeline)
   - [10. Transaction (MULTI/EXEC & Optimistic Locking WATCH)](#10-transaction)
   - [11. Stream (Event Streaming & Consumer Group)](#11-stream)
   - [12. Pub/Sub (Fire-and-Forget Messaging)](#12-pubsub)
4. [Cheat Sheet Pemilihan Struktur Data](#-cheat-sheet-pemilihan-struktur-data)
5. [Tabel Kompleksitas Waktu (Big-O)](#-tabel-kompleksitas-waktu-big-o)

---

## 🧠 Fundamental Arsitektur Redis

Sebelum masuk ke kode, penting untuk memahami bagaimana Redis bekerja di balik layar:

```text
+-----------------------------------------------------------+
|                        KLIEN GOLANG                       |
|   (Goroutine 1)       (Goroutine 2)       (Goroutine 3)   |
+---------+-------------------+-------------------+---------+
          |                   |                   |
          +-------------+     |     +-------------+
                        |     |     |
               +--------v-----v-----v-------+
               |  Connection Pool (TCP)     |
               +--------------+-------------+
                              |
                     Jaringan TCP Socket
                              |
               +--------------v-------------+
               |   I/O Multiplexing (epoll) |
               +--------------+-------------+
                              |
               +--------------v-------------+
               | Single-Threaded Event Loop | <--- Eksekusi perintah berurutan
               +--------------+-------------+      Bebas Race Condition internal!
                              |
               +--------------v-------------+
               |   RAM (In-Memory Engine)   |
               | String | List | Hash | ZSet|
               +--------------+-------------+
                              |
               +--------------v-------------+
               |   Persistensi: RDB / AOF   | <--- Simpan snapshot ke Disk
               +----------------------------+
```

1. **In-Memory Storage**: Seluruh data disimpan langsung di RAM. Waktu baca/tulis berada dalam skala mikrodetik (sub-millisecond).
2. **Single-Threaded Event Loop**: Redis memproses perintah satu per satu secara berurutan dalam single thread utama menggunakan I/O Multiplexing (`epoll` di Linux). Ini menjamin setiap perintah tunggal di Redis bersifat **atomik secara inheren** tanpa risiko race condition internal di server.
3. **Persistensi (RDB & AOF)**:
   - **RDB (Redis Database Backup)**: Snapshot point-in-time dari dataset yang disimpan secara berkala ke disk.
   - **AOF (Append Only File)**: Mencatat setiap log operasi tulis (write operation) yang diterima server.

---

## 🛠️ Prasyarat & Menjalankan Redis

### 1. Menjalankan Redis dengan Docker Compose
File [`docker-compose.yml`](docker-compose.yml) telah disesuaikan dengan konfigurasi port `6380` dan autentikasi password `password`:

```bash
# Menjalankan container Redis di background (port 6380)
docker compose up -d

# Menghentikan container Redis
docker compose down
```

### 2. Konfigurasi Variabel Lingkungan
Konfigurasi default proyek sudah otomatis menggunakan host `localhost:6380` dan password `password`. Jika Anda ingin mengubahnya:

```bash
export REDIS_ADDR=localhost:6380
export REDIS_PASSWORD=password
export REDIS_DB=0
```

### 3. Menjalankan Semua Pengujian
```bash
# Jalankan seluruh modul test sekaligus
go test -v ./...

# Jalankan dengan deteksi race condition Go
go test -race -v ./...
```

---

## 📚 Daftar 12 Modul Pembelajaran

### 01. Redis Client & Connection Pooling
📄 File: [`01_client_test.go`](01_client_test.go)

- **Konsep**:
  - `*redis.Client` di Go bertindak sebagai **singleton** yang mengelola sekumpulan koneksi TCP (Connection Pool) secara otomatis.
  - Setiap operasi pada `go-redis/v9` menerima parameter `context.Context` untuk deadline, timeout, dan tracing.
- **Perintah Penting**:
  - `client.Ping(ctx)`: Mengirim probe PING dan menerima PONG.
  - `client.PoolStats()`: Memantau jumlah koneksi aktif, idle, dan connection timeouts.
- **Perintah Menjalankan**:
  ```bash
  go test -v -run "TestClient"
  ```

---

### 02. String
📄 File: [`02_string_test.go`](02_string_test.go)

- **Konsep**:
  - Struktur data paling fundamental. Binary-safe (teks, JSON, file biner) hingga 512 MB.
  - Diimplementasikan dengan Simple Dynamic String (SDS).
- **Fitur yang Diuji**:
  - `Set` & `Get`: Operasi baca-tulis standar $O(1)$.
  - `redis.Nil`: Handling idiomatik Go untuk membedakan antara key tidak ada vs string kosong.
  - `SetEx` (TTL): Caching dengan masa berlaku kedaluwarsa.
  - `SetNX`: Set if Not Exists untuk mekanisme dasar **Distributed Lock**.
  - `Incr`, `IncrBy`, `Decr`: Counter atomik tanpa race condition.
  - `MSet`, `MGet`: Operasi batch multi-key untuk menghemat Round-Trip Time.
- **Perintah Menjalankan**:
  ```bash
  go test -v -run "TestString"
  ```

---

### 03. List
📄 File: [`03_list_test.go`](03_list_test.go)

- **Konsep**:
  - Kumpulan string terurut berdasarkan urutan masuk (insertion order).
  - Diimplementasikan dengan `quicklist` (gabungan doubly-linked list dan ziplist).
  - Sangat cepat $O(1)$ untuk penyisipan dan pengambilan di kedua ujung (HEAD dan TAIL).
- **Pola Penggunaan**:
  - **Queue (FIFO)**: `RPush` (masuk di ekor) dan `LPop` (keluar di kepala).
  - **Stack (LIFO)**: `RPush` dan `RPop`.
  - **Capped List**: `LPush` + `LTrim` (hanya menyimpan $N$ log terakhir agar memori tidak bengkak).
  - **Blocking Pop**: `BRPop` menunggu pesan baru dengan timeout tanpa polling CPU yang boros.
- **Perintah Menjalankan**:
  ```bash
  go test -v -run "TestList"
  ```

---

### 04. Set
📄 File: [`04_set_test.go`](04_set_test.go)

- **Konsep**:
  - Kumpulan string unik yang **tidak terurut** (unordered, no duplicates).
  - Pengecekan anggota `SIsMember` bernilai $O(1)$.
- **Operasi Aljabar Himpunan**:
  - `SInter` (Irisan): Mencari teman bersama (mutual friends) atau minat bersama.
  - `SUnion` (Gabungan): Menggabungkan kumpulan tag unik.
  - `SDiff` (Selisih): Menemukan rekomendasi item yang belum dimiliki pengguna.
  - `SRandMember` & `SPop`: Pemilihan pemenang undian/giveaway secara acak.
- **Perintah Menjalankan**:
  ```bash
  go test -v -run "TestSet"
  ```

---

### 05. Sorted Set (ZSet)
📄 File: [`05_sorted_set_test.go`](05_sorted_set_test.go)

- **Konsep**:
  - Elemen unik yang dipasangkan dengan nilai **skor numerik** (`float64`).
  - Elemen diurutkan secara otomatis dari skor terkecil hingga terbesar menggunakan struktur gabungan **Hash Table + Skip List** dengan kompleksitas $O(\log N)$.
- **Fitur yang Diuji**:
  - `ZAdd`: Menambahkan member dan skor.
  - `ZIncrBy`: Menambah skor secara atomik.
  - `ZRevRange`: Mengambil Top $N$ leaderboard game/skor tertinggi.
  - `ZRevRank`: Mengetahui peringkat user secara real-time.
  - `ZRangeByScore`: Filter data berdasarkan batas harga/skor minimum & maksimum.
- **Perintah Menjalankan**:
  ```bash
  go test -v -run "TestSortedSet"
  ```

---

### 06. Hash
📄 File: [`06_hash_test.go`](06_hash_test.go)

- **Konsep**:
  - Pemetaan field-value di dalam satu key (seperti `map[string]string` atau representasi `struct`).
  - **Hemat Memori**: Hash kecil disimpan sebagai `listpack`/`ziplist`, menghemat overhead RAM hingga 80% dibanding membuat key string terpisah.
- **Fitur yang Diuji**:
  - `HSet`, `HGet`, `HGetAll`, `HDel`.
  - `HIncrBy`: Menambah counter field secara atomik (misal: jumlah login).
  - `Scan(&struct)`: Unmarshaling otomatis dari Redis Hash ke struct Go dengan struct tag `redis:"field"`.
- **Perintah Menjalankan**:
  ```bash
  go test -v -run "TestHash"
  ```

---

### 07. GeoPoint (Geospatial)
📄 File: [`07_geopoint_test.go`](07_geopoint_test.go)

- **Konsep**:
  - Redis meng-encode koordinat lintang & bujur menjadi integer 52-bit (**Geohash**) dan menyimpannya di dalam **Sorted Set (ZSet)**.
  - ⚠️ **Perhatian**: Urutan koordinat di Redis adalah `(LONGITUDE, LATITUDE)` atau bujur dulu baru lintang.
- **Fitur yang Diuji**:
  - `GeoAdd`: Mendaftarkan titik lokasi.
  - `GeoDist`: Menghitung jarak garis lurus (km/meter) antar dua lokasi.
  - `GeoSearch`: Mencari semua lokasi di dalam radius tertentu (misal: radius 5 km dari user).
- **Perintah Menjalankan**:
  ```bash
  go test -v -run "TestGeoPoint"
  ```

---

### 08. HyperLogLog
📄 File: [`08_hyperloglog_test.go`](08_hyperloglog_test.go)

- **Konsep**:
  - Struktur data probabilistik untuk memperkirakan jumlah elemen unik (kardinalitas) dari kumpulan data raksasa.
  - **Efisiensi Memori Ekstrem**: Hanya mengonsumsi memori **maksimal ~12 KB**, terlepas dari apakah ada 100 data atau 100 juta data!
  - Standar error statistik: $\approx 0.81\%$.
- **Fitur yang Diuji**:
  - `PFAdd`: Menambahkan data (User ID / IP).
  - `PFCount`: Mengambil estimasi total unik (Unique Daily Active Users).
  - `PFMerge`: Menggabungkan metrik unik harian menjadi mingguan.
- **Perintah Menjalankan**:
  ```bash
  go test -v -run "TestHyperLogLog"
  ```

---

### 09. Pipeline
📄 File: [`09_pipeline_test.go`](09_pipeline_test.go)

- **Konsep**:
  - Mengirimkan sekumpulan perintah sekaligus dalam 1 paket TCP socket (batching) tanpa menunggu balasan satu per satu.
  - Mengurangi latency **Round-Trip Time (RTT)** jaringan secara drastis (peningkatan throughput 3x s/d 10x lipat).
  - ⚠️ **Catatan**: Pipeline standar murni optimasi jaringan, bukan transaksi atomik. Perintah client lain masih bisa disisipkan di sela-sela eksekusi batch.
- **Perintah Menjalankan**:
  ```bash
  go test -v -run "TestPipeline"
  ```

---

### 10. Transaction
📄 File: [`10_transaction_test.go`](10_transaction_test.go)

- **Konsep**:
  - Menjalankan sekumpulan perintah secara terisolasi dan atomik menggunakan blok `MULTI` dan `EXEC`.
  - Redis **tidak mendukung rollback runtime**; jika salah satu perintah salah tipe data, perintah lainnya tetap dijalankan.
- **Dua Pendekatan Transaksi**:
  1. `TxPipeline()`: Membungkus batch perintah dalam MULTI/EXEC.
  2. `client.Watch()`: **Optimistic Locking (Check-And-Set / CAS)** untuk membaca nilai, melakukan logika di aplikasi, lalu menulis kembali dengan jaminan tidak ada client lain yang memodifikasi key tersebut di tengah jalan (mencegah double-spending saldo atau overselling flash sale).
- **Perintah Menjalankan**:
  ```bash
  go test -v -run "TestTx"
  ```

---

### 11. Stream
📄 File: [`11_stream_test.go`](11_stream_test.go)

- **Konsep**:
  - Struktur data append-only log mutakhir untuk Event Streaming (mirip **Apache Kafka** versi ringan).
  - Pesan tersimpan persisten (tidak hilang saat worker offline).
  - Setiap pesan memiliki ID berbasis timestamp: `<timestamp_ms>-<sequence>`.
- **Fitur Consumer Group**:
  - Membagi beban pesan ke worker secara merata (Load Balancing).
  - Konfirmasi pemrosesan dengan `XAck`.
  - Melacak pesan yang menggantung / worker crash via Pending Entries List (PEL).
- **Perintah Menjalankan**:
  ```bash
  go test -v -run "TestStream"
  ```

---

### 12. Pub/Sub
📄 File: [`12_pubsub_test.go`](12_pubsub_test.go)

- **Konsep**:
  - Pola pesan Publish/Subscribe decoupled.
  - **Fire-and-Forget**: Pesan bersifat efemeral. Jika tidak ada subscriber aktif saat pesan dikirim, pesan langsung dibuang.
  - Sangat cepat dengan latensi sub-milidetik untuk broadcast real-time.
- **Fitur yang Diuji**:
  - `Publish` & `Subscribe`: Komunikasi melalui channel tertentu.
  - `PSubscribe`: Berlangganan menggunakan pola regex/wildcard (misal: `sensor.*.temperature`).
- **Perintah Menjalankan**:
  ```bash
  go test -v -run "TestPubSub"
  ```

---

## 🎯 Cheat Sheet Pemilihan Struktur Data

| Kebutuhan Aplikasi | Struktur Data Redis Terbaik | Alternatif |
| :--- | :--- | :--- |
| Caching JSON API / Token Sesi | **String** | Hash |
| Counter Atomik (Hit Counter / Rate Limit) | **String (`INCR`)** | Hash (`HINCRBY`) |
| Distributed Lock / Idempotency Key | **String (`SETNX`)** | Redlock |
| Antrean Tugas Sederhana (Job Queue) | **List (`RPUSH` + `BLPOP`)** | Stream |
| Riwayat Log Terbatas (Capped Buffer) | **List (`LPUSH` + `LTRIM`)** | Stream (`XTRIM`) |
| Kumpulan Nilai Unik / Tagging / Blacklist | **Set (`SADD`)** | Hash |
| Rekomendasi Teman / Mutual Friends | **Set (`SINTER`)** | Graph |
| Papan Skor / Leaderboard Game | **Sorted Set (`ZADD`)** | SQL Index |
| Filter Produk Berdasarkan Rentang Harga | **Sorted Set (`ZRANGEBYSCORE`)** | SQL Index |
| Entitas Objek / Struct (User, Produk) | **Hash (`HSET`, `HGETALL`)** | String (JSON) |
| Pencarian Driver / Toko Terdekat | **GeoPoint (`GEOSEARCH`)** | PostGIS |
| Menghitung Pengunjung Unik Skala Besar (DAU) | **HyperLogLog (`PFADD`, `PFCOUNT`)** | Set (boros RAM) |
| Event Sourcing & Reliable Asynchronous Worker | **Stream (Consumer Group + `XACK`)** | RabbitMQ / Kafka |
| Chat Real-time & WebSocket Broadcast | **Pub/Sub (`PUBLISH`)** | Stream |

---

## ⏱️ Tabel Kompleksitas Waktu (Big-O)

| Operasi | Tipe Data | Kompleksitas Waktu | Keterangan |
| :--- | :--- | :--- | :--- |
| `GET`, `SET` | String | $O(1)$ | Sangat cepat |
| `INCR`, `DECR` | String | $O(1)$ | Atomik di tingkat server |
| `LPUSH`, `LPOP` | List | $O(1)$ | Operasi di ujung list instan |
| `LRANGE` | List | $O(S + N)$ | $S$ offset awal, $N$ jumlah elemen |
| `SADD`, `SISMEMBER` | Set | $O(1)$ | Pengecekan anggota instan |
| `SINTER` | Set | $O(N \times M)$ | $N$ himpunan terkecil, $M$ jumlah himpunan |
| `ZADD` | Sorted Set | $O(\log N)$ | Disisipkan ke Skip List |
| `ZRANK`, `ZREVRANK`| Sorted Set | $O(\log N)$ | Pencarian posisi peringkat |
| `HGET`, `HSET` | Hash | $O(1)$ | Akses field instan |
| `GEOSEARCH` | Geo | $O(N + \log M)$ | $N$ titik dalam area, $M$ total data |
| `PFADD`, `PFCOUNT` | HyperLogLog | $O(1)$ | Operasi register konstan |
| `XADD` | Stream | $O(1)$ | Penambahan append-only instan |
| `XREADGROUP` | Stream | $O(M)$ | $M$ jumlah pesan yang diambil |

---

## 🛡️ Best Practices Penggunaan Redis di Production

1. **Gunakan Connection Pooling**: Jangan membuat `redis.NewClient` baru pada setiap fungsi controller/handler HTTP. Selalu gunakan satu client global (singleton).
2. **Tetapkan Nilai TTL pada Cache**: Hindari key cache tanpa expiration agar memory tidak penuh secara diam-diam.
3. **Konfigurasi Eviction Policy**: Atur parameter `maxmemory` dan `maxmemory-policy` (misal: `volatile-lru` atau `allkeys-lru`) pada file `redis.conf`.
4. **Hindari Perintah Berbahaya di Production**: Jangan pernah menjalankan perintah `KEYS *` pada dataset besar di production karena memblokir single-threaded event loop Redis. Gunakan `SCAN` sebagai gantinya.
5. **Gunakan Pipeline untuk Batch Workload**: Jika memproses lebih dari 10 query sekaligus, selalu pertimbangkan `Pipeline` untuk memangkas latensi jaringan.
