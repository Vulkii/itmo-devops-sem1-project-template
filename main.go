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
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Fail to load the file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	tempDir := "./temp"
	os.MkdirAll(tempDir, os.ModePerm)

	zipPath := filepath.Join(tempDir, header.Filename)
	outFile, err := os.Create(zipPath)
	if err != nil {
		http.Error(w, "Fail to create outFile", http.StatusInternalServerError)
		return
	}
	defer outFile.Close()
	io.Copy(outFile, file)

	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		http.Error(w, "Fail to read the archive", http.StatusInternalServerError)
		return
	}
	defer zipReader.Close()

	var totalItems, totalPrice int
	categories := make(map[string]bool)

	for _, f := range zipReader.File {
		if strings.HasSuffix(f.Name, ".csv") {
			processCSV(f, &totalItems, &totalPrice, categories)
		}
	}

	response := map[string]interface{}{
		"total_items":      totalItems,
		"total_categories": len(categories),
		"total_price":      totalPrice,
	}
	json.NewEncoder(w).Encode(response)
}

func processCSV(f *zip.File, totalItems *int, totalPrice *int, categories map[string]bool) {
	rc, _ := f.Open()
	defer rc.Close()

	reader := csv.NewReader(rc)
	reader.Read()

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}

		productID, _ := strconv.Atoi(record[0])
		createdAt := record[1]
		name := record[2]
		category := record[3]
		price, _ := strconv.Atoi(record[4])

		categories[category] = true
		*totalItems++
		*totalPrice += price

		db.Exec("INSERT INTO prices (product_id, created_at, name, category, price) VALUES ($1, $2, $3, $4, $5)",
			productID, createdAt, name, category, price)
	}
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
