package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/robfig/cron/v3"
)

type Service struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // "http", "tcp", or "ping"
	URL  string `json:"url"`
}

type UptimeLog struct {
	ID        int       `json:"id"`
	ServiceID int       `json:"service_id"`
	Up        bool      `json:"up"`
	CheckedAt time.Time `json:"checked_at"`
}

var db *sql.DB

func main() {
	var err error
	db, err = sql.Open("sqlite3", "uptime.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	initDB()

	c := cron.New()
	c.AddFunc("@every 1m", checkAllServices)
	c.Start()

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/api/services", handleServices)
	http.HandleFunc("/api/uptime", handleUptime)
	http.HandleFunc("/", serveIndex)

	log.Println("Server started on :8080")
	http.ListenAndServe(":8080", nil)
}

func initDB() {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS services (
			   id INTEGER PRIMARY KEY AUTOINCREMENT,
			   name TEXT,
			   type TEXT,
			   url TEXT,
			   position INTEGER DEFAULT 0
	   );`)
	if err != nil {
		log.Fatal(err)
	}
	// Add position column if it doesn't exist (for migrations)
	db.Exec(`ALTER TABLE services ADD COLUMN position INTEGER DEFAULT 0`)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS uptime_logs (
			   id INTEGER PRIMARY KEY AUTOINCREMENT,
			   service_id INTEGER,
			   up INTEGER,
			   checked_at DATETIME,
			   FOREIGN KEY(service_id) REFERENCES services(id)
	   );`)
	if err != nil {
		log.Fatal(err)
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	f, err := os.Open("static/index.html")
	if err != nil {
		w.WriteHeader(404)
		w.Write([]byte("Not found"))
		return
	}
	defer f.Close()
	stat, _ := f.Stat()
	w.Header().Set("Content-Type", "text/html")
	http.ServeContent(w, r, "index.html", stat.ModTime(), f)
}

func handleServices(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		rows, err := db.Query("SELECT id, name, type, url FROM services ORDER BY position ASC, id ASC")
		if err != nil {
			w.WriteHeader(500)
			return
		}
		defer rows.Close()
		var services []Service
		for rows.Next() {
			var s Service
			rows.Scan(&s.ID, &s.Name, &s.Type, &s.URL)
			services = append(services, s)
		}
		json.NewEncoder(w).Encode(services)
		return
	}
	if r.Method == http.MethodPost {
		var s Service
		json.NewDecoder(r.Body).Decode(&s)
		// Set position to max+1
		var maxPos int
		db.QueryRow("SELECT COALESCE(MAX(position), 0) FROM services").Scan(&maxPos)
		_, err := db.Exec("INSERT INTO services (name, type, url, position) VALUES (?, ?, ?, ?)", s.Name, s.Type, s.URL, maxPos+1)
		if err != nil {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(201)
		return
	}
	if r.Method == http.MethodPut {
		// Update order: expects JSON array of service IDs in new order
		var ids []int
		if err := json.NewDecoder(r.Body).Decode(&ids); err != nil {
			w.WriteHeader(400)
			w.Write([]byte("Invalid body"))
			return
		}
		for pos, id := range ids {
			db.Exec("UPDATE services SET position = ? WHERE id = ?", pos, id)
		}
		w.WriteHeader(204)
		return
	}
	if r.Method == http.MethodDelete {
		id := r.URL.Query().Get("id")
		if id == "" {
			w.WriteHeader(400)
			w.Write([]byte("Missing id"))
			return
		}
		// Remove uptime logs for this service first (optional, for DB integrity)
		db.Exec("DELETE FROM uptime_logs WHERE service_id = ?", id)
		_, err := db.Exec("DELETE FROM services WHERE id = ?", id)
		if err != nil {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(204)
		return
	}
}

func handleUptime(w http.ResponseWriter, r *http.Request) {
	serviceID := r.URL.Query().Get("service_id")
	rows, err := db.Query("SELECT id, service_id, up, checked_at FROM uptime_logs WHERE service_id = ? ORDER BY checked_at DESC LIMIT 100", serviceID)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	defer rows.Close()
	var logs []UptimeLog
	for rows.Next() {
		var l UptimeLog
		var upInt int
		var checkedAt string
		rows.Scan(&l.ID, &l.ServiceID, &upInt, &checkedAt)
		l.Up = upInt == 1
		l.CheckedAt, _ = time.Parse(time.RFC3339, checkedAt)
		logs = append(logs, l)
	}
	json.NewEncoder(w).Encode(logs)
}

func checkAllServices() {
	rows, err := db.Query("SELECT id, name, type, url FROM services")
	if err != nil {
		log.Println("Failed to fetch services:", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var s Service
		rows.Scan(&s.ID, &s.Name, &s.Type, &s.URL)
		go checkService(s)
	}
}

func checkService(s Service) {
	up := false
	if s.Type == "http" {
		client := http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(s.URL)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 400 {
			up = true
		}
		if resp != nil {
			resp.Body.Close()
		}
	} else if s.Type == "tcp" {
		conn, err := net.DialTimeout("tcp", s.URL, 5*time.Second)
		if err == nil {
			up = true
			conn.Close()
		}
	} else if s.Type == "ping" {
		up = pingHost(s.URL)
	}
	db.Exec("INSERT INTO uptime_logs (service_id, up, checked_at) VALUES (?, ?, ?)", s.ID, boolToInt(up), time.Now().Format(time.RFC3339))
}

// pingHost sends a single ICMP echo request to the host and returns true if it receives a reply.
func pingHost(host string) bool {
	// Use exec to call system ping for cross-platform support (1 echo, 2s timeout)
	// On Windows: ping -n 1 -w 2000 host
	// On Unix: ping -c 1 -W 2 host
	var cmd *exec.Cmd
	// timeout := 2 * time.Second // not used
	if runtime.GOOS == "windows" {
		cmd = exec.Command("ping", "-n", "1", "-w", "2000", host)
	} else {
		cmd = exec.Command("ping", "-c", "1", "-W", "2", host)
	}
	err := cmd.Run()
	return err == nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
