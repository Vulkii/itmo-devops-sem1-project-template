package main

import (
	"archive/zip"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
	file, _, err := r.FormFile("file")
	if err != nil {
		log.Printf("Ошибка загрузки файла: %v", err)
		http.Error(w, "Не удалось загрузить файл", http.StatusBadRequest)
		return
	}
	defer file.Close()

	tempFile, err := os.CreateTemp("", "uploaded-*.zip")
	if err != nil {
		log.Printf("Ошибка сохранения файла: %v", err)
		http.Error(w, "Ошибка сохранения файла", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile.Name())

	if _, err := io.Copy(tempFile, file); err != nil {
		log.Printf("Ошибка копирования файла: %v", err)
		http.Error(w, "Ошибка копирования файла", http.StatusInternalServerError)
		return
	}

	zipReader, err := zip.OpenReader(tempFile.Name())
	if err != nil {
		log.Printf("Ошибка открытия архива: %v", err)
		http.Error(w, "Ошибка чтения архива", http.StatusBadRequest)
		return
	}
	defer zipReader.Close()

	var csvRecords [][]string
	for _, f := range zipReader.File {
		if strings.HasSuffix(f.Name, ".csv") {
			csvFile, err := f.Open()
			if err != nil {
				log.Printf("Ошибка открытия CSV: %v", err)
				continue
			}
			defer csvFile.Close()

			reader := csv.NewReader(csvFile)
			records, err := reader.ReadAll()
			if err != nil {
				log.Printf("Ошибка чтения CSV: %v", err)
				continue
			}
			csvRecords = append(csvRecords, records...)
		}
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("Ошибка начала транзакции: %v", err)
		http.Error(w, "Ошибка начала транзакции", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	for _, record := range csvRecords {
		if len(record) < 5 {
			continue
		}

		productID, err := strconv.Atoi(record[0])
		if err != nil {
			log.Printf("Ошибка преобразования product_id: %v", err)
			continue
		}
		createdAt := record[1]
		name := record[2]
		category := record[3]
		price, err := strconv.ParseFloat(record[4], 64)
		if err != nil {
			log.Printf("Ошибка преобразования цены: %v", err)
			continue
		}

		if _, err := time.Parse("2006-01-02", createdAt); err != nil {
			log.Printf("Некорректный формат даты '%s': %v", createdAt, err)
			continue
		}

		_, err = tx.Exec(`INSERT INTO prices (product_id, created_at, name, category, price) VALUES ($1, $2, $3, $4, $5)`,
			productID, createdAt, name, category, price)
		if err != nil {
			log.Printf("Ошибка вставки в БД: %v", err)
			continue
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Ошибка подтверждения транзакции: %v", err)
		http.Error(w, "Ошибка подтверждения транзакции", http.StatusInternalServerError)
		return
	}
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
