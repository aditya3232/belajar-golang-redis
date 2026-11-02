package belajar_golang_redis

import (
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// TOPIC 7: REDIS GEOPOINT (GEOSPATIAL) FUNDAMENTALS
// ============================================================================
// Fundamental:
// Fitur Geospasial Redis memungkinkan penyimpanan koordinat garis lintang (latitude)
// dan bujur (longitude), serta query jarak dan pencarian radius terdekat.
//
// Rahasia Implementasi Internal:
// Redis TIDAK memiliki tipe data baru khusus untuk Geo!
// Di balik layar, koordinat (lon, lat) di-encode menjadi integer 52-bit menggunakan
// algoritma `Geohash`, dan disimpan di dalam struktur data **SORTED SET (ZSET)**.
// Nilai geohash tersebut menjadi `score`, dan nama lokasi menjadi `member`.
//
// ⚠️ PERINGATAN PENTING (Beginner Pitfall):
// Urutan koordinat di Redis adalah: (LONGITUDE, LATITUDE) atau (Bujur, Lintang).
// JANGAN TERBALIK! (Bujur/X dulu baru Lintang/Y).
// Batas koordinat:
// - Longitude: -180 s/d 180 derajat
// - Latitude: -85.05112878 s/d 85.05112878 derajat
//
// Use Cases Populer:
// 1. Aplikasi Ride-Hailing (Mencari driver ojek online terdekat dari penumpang).
// 2. Pencarian Restoran / ATM / Cabang Toko terdekat dalam radius X km.
// 3. Perhitungan ongkos kirim berbasis jarak (km).
// ============================================================================

func TestGeoPointOperations(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	geoKey := UniqueKey("locations:jakarta")
	CleanKey(t, client, ctx, geoKey)

	// Skenario: Menambahkan koordinat lokasi populer di Jakarta
	// 1. Monas (Monumen Nasional): Lon 106.827153, Lat -6.175392
	// 2. Bundaran HI: Lon 106.823055, Lat -6.195045
	// 3. Gelora Bung Karno (GBK): Lon 106.801648, Lat -6.218556
	// 4. Bandara Soekarno Hatta: Lon 106.655897, Lat -6.125556

	// 1. GEOADD: Menambahkan lokasi (Longitude, Latitude, Name)
	added, err := client.GeoAdd(ctx, geoKey,
		&redis.GeoLocation{Name: "Monas", Longitude: 106.827153, Latitude: -6.175392},
		&redis.GeoLocation{Name: "Bundaran HI", Longitude: 106.823055, Latitude: -6.195045},
		&redis.GeoLocation{Name: "GBK", Longitude: 106.801648, Latitude: -6.218556},
		&redis.GeoLocation{Name: "Bandara Soetta", Longitude: 106.655897, Latitude: -6.125556},
	).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(4), added)

	// 2. GEOPOS: Mengambil kembali koordinat yang tersimpan
	positions, err := client.GeoPos(ctx, geoKey, "Monas").Result()
	assert.NoError(t, err)
	assert.Len(t, positions, 1)
	// Geohash memiliki sedikit toleransi presisi (kurang dari 1 meter deviasi)
	assert.InDelta(t, 106.827153, positions[0].Longitude, 0.0001)
	assert.InDelta(t, -6.175392, positions[0].Latitude, 0.0001)

	// 3. GEODIST: Menghitung jarak garis lurus antara 2 titik
	// Satuan yang didukung: "m" (meter), "km" (kilometer), "mi" (miles), "ft" (feet).
	distKm, err := client.GeoDist(ctx, geoKey, "Monas", "Bundaran HI", "km").Result()
	assert.NoError(t, err)
	// Jarak Monas ke Bundaran HI adalah sekitar 2.2 km
	assert.True(t, distKm > 2.0 && distKm < 2.5, "Jarak Monas ke Bundaran HI harus sekitar 2.2 km")
	t.Logf("Jarak Monas -> Bundaran HI: %.2f km", distKm)

	// 4. GEOSEARCH: Mencari lokasi di sekitar titik koordinat tertentu dalam radius
	// Misal: Pengguna sedang berada di sekitar Sudirman (Lon 106.822, Lat -6.198)
	// dan ingin mencari tempat populer dalam radius 5 KM dengan urutan terdekat (ASC).
	locations, err := client.GeoSearch(ctx, geoKey, &redis.GeoSearchQuery{
		Longitude:  106.822,
		Latitude:   -6.198,
		Radius:     5.0,
		RadiusUnit: "km",
		Sort:       "ASC", // Urutkan dari yang paling dekat
	}).Result()
	assert.NoError(t, err)

	// Dalam radius 5 km, Bundaran HI, GBK, dan Monas masuk cakupan.
	// Bandara Soetta (> 20 km) tidak boleh masuk.
	assert.Contains(t, locations, "Bundaran HI")
	assert.Contains(t, locations, "Monas")
	assert.NotContains(t, locations, "Bandara Soetta")

	// 5. GEOSEARCH WITH DETAIL: Mengambil nama sekaligus jarak persisnya
	detailResults, err := client.GeoSearchLocation(ctx, geoKey, &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Member:     "Monas", // Cari di sekitar member "Monas"
			Radius:     10.0,
			RadiusUnit: "km",
			Sort:       "ASC",
		},
		WithDist:  true, // Minta Redis menyertakan jarak
		WithCoord: true, // Minta koordinat
	}).Result()
	assert.NoError(t, err)

	t.Log("Hasil Pencarian Lokasi di sekitar Monas:")
	for _, loc := range detailResults {
		t.Logf("- %s: Jarak = %.2f km, Koordinat = (Lon: %f, Lat: %f)",
			loc.Name, loc.Dist, loc.Longitude, loc.Latitude)
	}
}
