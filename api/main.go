package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/google/generative-ai-go/genai"
	_ "github.com/lib/pq"
	"google.golang.org/api/option"
)

var db *sql.DB

// --- STRUCTS ---
type Kampus struct {
	ID   int    `json:"id"`
	Nama string `json:"nama"`
}

type Jurusan struct {
	ID          int     `json:"id"`
	NamaJurusan string  `json:"nama"`
	SkorMinimal float64 `json:"min"`
	SkorAman    float64 `json:"aman"`
}

type RequestAI struct {
	Skor   float64 `json:"skor"`
	Target string  `json:"target"`
}

func main() {
	initDB()
	defer db.Close()

	// --- ROUTING ---
	http.HandleFunc("/api/kampus", getKampusHandler)
	http.HandleFunc("/api/jurusan", getJurusanHandler)
	http.HandleFunc("/api/rekomendasi-ai", getAIRecommendationHandler)

	fmt.Println("🚀 Server Golang jalan di port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// --- FUNGSI KONEKSI DATABASE ---
func initDB() {
	var err error
	// Ganti "postgres" dan "password_lu" sesuai setup lokal di Linux lu
	// Ganti baris connStr di main.go lu jadi begini:
	connStr := "postgres://postgres:pass-utbk26_Skywalker51%23@db.aauajjwjjmokggheytih.supabase.co:5432/postgres"
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Gagal konek DB: ", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatal("DB nggak respon: ", err)
	}
	fmt.Println("✅ Database PostgreSQL Terkoneksi!")
}

// --- HANDLER 1: Ambil List Kampus ---
func getKampusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // Biar ga kena CORS error di frontend

	rows, err := db.Query("SELECT id, nama_kampus FROM kampus ORDER BY nama_kampus ASC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var kampuses []Kampus
	for rows.Next() {
		var k Kampus
		if err := rows.Scan(&k.ID, &k.Nama); err != nil {
			log.Println(err)
			continue
		}
		kampuses = append(kampuses, k)
	}
	json.NewEncoder(w).Encode(kampuses)
}

// --- HANDLER 2: Ambil Jurusan berdasarkan Kampus ID ---
func getJurusanHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	kampusID := r.URL.Query().Get("kampus_id")
	if kampusID == "" {
		http.Error(w, "kampus_id wajib diisi", http.StatusBadRequest)
		return
	}

	query := "SELECT id, nama_jurusan, skor_minimal, skor_aman FROM jurusan_kampus WHERE kampus_id = $1 ORDER BY nama_jurusan ASC"
	rows, err := db.Query(query, kampusID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var jurusans []Jurusan
	for rows.Next() {
		var j Jurusan
		if err := rows.Scan(&j.ID, &j.NamaJurusan, &j.SkorMinimal, &j.SkorAman); err != nil {
			log.Println(err)
			continue
		}
		jurusans = append(jurusans, j)
	}
	json.NewEncoder(w).Encode(jurusans)
}

// --- HANDLER 3: Tembak Gemini API ---
func getAIRecommendationHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight request buat CORS
	if r.Method == "OPTIONS" {
		return
	}

	var reqData RequestAI
	if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	// Pastikan lu udah set GEMINI_API_KEY di environment variables Linux lu
	client, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
	if err != nil {
		http.Error(w, "Gagal inisialisasi AI", http.StatusInternalServerError)
		return
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.5-flash")
	
	prompt := fmt.Sprintf("Seorang siswa mendapatkan skor UTBK %f dan mendaftar di prodi %s. Berikan maksimal 2 kalimat rekomendasi singkat mengenai 2 alternatif jurusan lain (satu setara, satu dibawahnya) yang masih relevan. Gunakan bahasa yang santai.", reqData.Skor, reqData.Target)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		http.Error(w, "Gagal generate AI", http.StatusInternalServerError)
		return
	}

	// Ekstrak teks dari respon Gemini
	var aiResponseText string
	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				aiResponseText += fmt.Sprintf("%v", part)
			}
		}
	}

	// Kirim balik ke frontend dalam bentuk JSON
	json.NewEncoder(w).Encode(map[string]string{"saran": aiResponseText})
}