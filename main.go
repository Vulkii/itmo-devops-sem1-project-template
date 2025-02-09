package main

import (
	"archive/zip"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
)

const (
	dbUser     = "validator"
	dbPassword = "val1dat0r"
	dbName     = "project-sem-1"
	dbHost     = "localhost"
	dbPort     = "5432"
)

type ResponseonPost struct {
	TotalItems      int     `json:"total_items"`
	TotalCategories int     `json:"total_categories"`
	TotalPrice      float64 `json:"total_price"`
}

var db *sql.DB

func main() {
	var err error
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Fail to connect to DB", err)
	}
	defer db.Close()

	http.HandleFunc("/api/v0/prices", handleRequests)
	log.Println("Server has started")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleRequests(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		handlePost(w, r)
	case "GET":
		handleGet(w, r)
	default:
		http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
	}
}

func handlePost(w http.ResponseWriter, r *http.Request) {
	log.Println("Got POST-request.")

	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("Fail to load the file: %v\n", err)
		http.Error(w, "Fail to load the file", http.StatusBadRequest)
		return
	}
	defer file.Close()
	log.Printf("File %s uploaded sucessfully.\n", header.Filename)

	tempDir := "./temp"
	os.MkdirAll(tempDir, os.ModePerm)

	zipPath := filepath.Join(tempDir, header.Filename)
	outFile, err := os.Create(zipPath)
	if err != nil {
		log.Printf("Fail to create outFile %s: %v\n", zipPath, err)
		http.Error(w, "Fail to create outFile", http.StatusInternalServerError)
		return
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, file)
	if err != nil {
		log.Printf("Fail to save file: %v\n", err)
		http.Error(w, "Fail to save file", http.StatusInternalServerError)
		return
	}
	log.Printf("File saved in temp dir: %s\n", zipPath)

	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		log.Printf("Fail to read the archive: %v\n", err)
		http.Error(w, "Fail to read the archive", http.StatusInternalServerError)
		return
	}
	defer zipReader.Close()

	var totalItems int
	var totalPrice float64
	categories := make(map[string]bool)

	for _, f := range zipReader.File {
		if strings.HasSuffix(f.Name, ".csv") {
			log.Printf("CSV detected: %s\n", f.Name)
			processCSV(f, &totalItems, &totalPrice, categories)
		}
	}

	response := ResponseonPost{
		TotalItems:      totalItems,
		TotalCategories: len(categories),
		TotalPrice:      totalPrice,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("JSON error: %v", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
	}

}

func processCSV(f *zip.File, totalItems *int, totalPrice *float64, categories map[string]bool) {
	log.Printf("Starting CSV: %s\n", f.Name)

	rc, err := f.Open()
	if err != nil {
		log.Printf("Fail to open CSV %s: %v\n", f.Name, err)
		return
	}
	defer rc.Close()

	reader := csv.NewReader(rc)

	header, err := reader.Read()
	if err != nil {
		log.Printf("Fail to read CSV: %v\n", err)
		return
	}
	log.Printf("Heading CSV: %v\n", header)

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Fail to read string in CSV: %v\n", err)
			continue
		}

		productID, err := strconv.Atoi(strings.TrimSpace(record[0]))
		if err != nil {
			log.Printf("Fail to rework product_id '%s': %v\n", record[0], err)
			continue
		}

		createdAt := strings.TrimSpace(record[4])
		name := strings.TrimSpace(record[1])
		category := strings.TrimSpace(record[2])

		price, err := strconv.ParseFloat(strings.TrimSpace(record[3]), 64)
		if err != nil {
			log.Printf("Fail to reworkы '%s': %v\n", record[3], err)
			continue
		}

		log.Printf("String: ID=%d, Name=%s, Category=%s, Price=%.2f, Date=%s\n",
			productID, name, category, price, createdAt)

		categories[category] = true
		*totalItems++
		*totalPrice += price

		_, err = db.Exec("INSERT INTO prices (product_id, created_at, name, category, price) VALUES ($1, $2, $3, $4, $5)",
			productID, createdAt, name, category, price)
		if err != nil {
			log.Printf("Ошибка записи в базу данных для ID %d: %v\n", productID, err)
			continue
		}
	}

	log.Printf("CSV ready. %s. Result: totalItems=%d, totalPrice=%.2f, totalCategories=%d\n",
		f.Name, *totalItems, *totalPrice, len(categories))
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query("SELECT product_id, created_at, name, category, price FROM prices")
	defer rows.Close()

	tempDir := "./temp"
	os.MkdirAll(tempDir, os.ModePerm)
	csvFilePath := filepath.Join(tempDir, "data.csv")
	csvFile, _ := os.Create(csvFilePath)
	defer csvFile.Close()

	writer := csv.NewWriter(csvFile)
	writer.Write([]string{"id", "created_at", "name", "category", "price"})

	for rows.Next() {
		var id int
		var createdAt, name, category string
		var price int
		rows.Scan(&id, &createdAt, &name, &category, &price)
		writer.Write([]string{strconv.Itoa(id), createdAt, name, category, strconv.Itoa(price)})
	}
	writer.Flush()

	zipFilePath := filepath.Join(tempDir, "data.zip")
	zipFile, _ := os.Create(zipFilePath)
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	fileInZip, _ := zipWriter.Create("data.csv")
	csvBytes, _ := os.ReadFile(csvFilePath)
	fileInZip.Write(csvBytes)
	zipWriter.Close()

	w.Header().Set("Content-Disposition", "attachment; filename=data.zip")
	w.Header().Set("Content-Type", "application/zip")
	http.ServeFile(w, r, zipFilePath)
}
