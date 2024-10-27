package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

type File struct {
	ID          int
	Title       string
	Description string
	Format      string
	Size        int64
	Path        string
	UploadTime  string
}

const uploadDir = "./uploads/"

func main() {
	_ = os.MkdirAll(uploadDir, os.ModePerm)

	http.Handle("/", http.FileServer(http.Dir(".")))

	http.HandleFunc("/upload", uploadHandler)

	http.HandleFunc("/files", fileHandler)

	fmt.Println("Server started at :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println("Error starting server:", err)
	}
}

// connectToDatabase connects to the database and returns the connection
func connectToDatabase() (*sql.DB, error) {
	dsn := "root:my-secret-pw@tcp(localhost:3306)/master"

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Println("Error opening the database:", err)
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		fmt.Println("Error connecting to the database:", err)
		return nil, err
	}

	fmt.Println("Connected to the database!")
	return db, nil
}

// insertDataIntoDataBase inserts the file metadata into the database
func insertDataIntoDataBase(db *sql.DB, title string, description string,
	format string, size int64, path string) error {
	query := "INSERT INTO Files (title, description, format, size, path) VALUES (?, ?, ?, ?, ?)"
	_, err := db.Exec(query, title, description, format, size, path)
	if err != nil {
		fmt.Println("Error inserting file into database:", err)
		return err
	}

	return nil
}

// uploadHandler handles the file upload
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10 << 20) // 10 MB

	if err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Unable to retrieve file", http.StatusBadRequest)
		return
	}

	defer file.Close()

	filename := r.FormValue("filename")
	description := r.FormValue("description")
	format := header.Header.Get("Content-Type")
	extension := filepath.Ext(header.Filename)
	size := header.Size
	path := uploadDir + filename + extension
	fmt.Println(extension)

	dst, err := os.Create(uploadDir + filename + extension)
	if err != nil {
		http.Error(w, "Unable to create file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Unable to save file", http.StatusInternalServerError)
		return
	}

	fmt.Fprintln(w, "File uploaded successfully")

	db, err := connectToDatabase()
	if err != nil {
		fmt.Println("Error connecting to database:", err)
		return
	}

	defer db.Close()

	err = insertDataIntoDataBase(db, filename, description, format, size, path)
	if err != nil {
		fmt.Println("Error inserting file into database:", err)
		return
	}

	fmt.Println("File inserted into database")
}

// fileHandler handles the file retrieval and displays the file information in the browser
func fileHandler(w http.ResponseWriter, r *http.Request) {
	db, err := connectToDatabase()
	if err != nil {
		fmt.Println("Error connecting to database:", err)
		return
	}

	defer db.Close()

	rows, err := db.Query("SELECT * FROM Files")
	if err != nil {
		fmt.Println("Error querying database:", err)
		return
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var file File
		err := rows.Scan(&file.ID, &file.Title, &file.Description, &file.Format,
			&file.Size, &file.Path, &file.UploadTime)
		if err != nil {
			fmt.Println("Error scanning row:", err)
			return
		}

		files = append(files, file)
	}

	fmt.Println("Files retrieved from database")

	w.Header().Set("Content-Type", "text/html")
	for _, file := range files {
		fmt.Fprintf(w, `<div class="file-item">
            <h2>%s</h2>
            <p>%s</p>
            <p>Format: %s</p>
            <p>Size: %d bytes</p>
            <a href="%s" download>Download</a>
        </div>`, file.Title, file.Description, file.Format, file.Size, file.Path)
	}
}
