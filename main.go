package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Config struct {
	MongoDB struct {
		URI      string `json:"uri"`
		Database string `json:"database"`
	} `json:"mongodb"`
	Server struct {
		Port int    `json:"port"`
		Host string `json:"host"`
	} `json:"server"`
	Upload struct {
		MaxSize int64  `json:"maxSize"`
		BaseURL string `json:"baseURL"`
	} `json:"upload"`
}

var (
	client    *mongo.Client
	gfsBucket *gridfs.Bucket
	config    Config
)

// =-=-=-=-=-=-=-=-XYLIUPLOADER-=-=-=-=-=-=-=-=

// Привет. Это Манук. Пару слов о проекте: Здесь используется база данных MongoDB для хранения файлов в GridFS.
// Каждый файл получает уникальный короткий идентификатор для доступа и отдельный токен для удаления.
// Веб-сервер обрабатывает загрузку, просмотр и удаление файлов через HTTP эндпоинты.
// В example.config.json указаны основные настройки, такие как подключение к базе данных и ограничения на загрузку файлов.
// Если разберётесь - красавы) Удачи!

// =-=-=-=-=-=-=-=-BY=MANUKQ-=-=-=-=-=-=-=-=-=

func init() {
	godotenv.Load()

	configFile, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatal("Error reading config.json:", err)
	}

	err = json.Unmarshal(configFile, &config)
	if err != nil {
		log.Fatal("Error parsing config.json:", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err = mongo.Connect(ctx, options.Client().ApplyURI(config.MongoDB.URI))
	if err != nil {
		log.Fatal("Error connecting to MongoDB:", err)
	}

	db := client.Database(config.MongoDB.Database)
	gfsBucket, err = gridfs.NewBucket(db)
	if err != nil {
		log.Fatal("Error creating GridFS bucket:", err)
	}

	log.Printf("Connected to MongoDB at %s", config.MongoDB.URI)
	log.Printf("Using database: %s", config.MongoDB.Database)
}

func generateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:5]
}

func getFileType(contentType string) string {
	if strings.HasPrefix(contentType, "image/") {
		return "image"
	}
	if strings.HasPrefix(contentType, "video/") {
		return "video"
	}
	if strings.HasPrefix(contentType, "audio/") {
		return "audio"
	}
	return "file"
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func main() {
	defer client.Disconnect(context.Background())

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/favicon.ico")
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			tmpl := template.Must(template.ParseFiles("templates/index.html"))
			err := tmpl.Execute(w, nil)
			if err != nil {
				http.Error(w, "template error", http.StatusInternalServerError)
			}
			return
		}

		fileID := r.URL.Path[1:]
		if fileID == "" {
			http.NotFound(w, r)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cursor, err := gfsBucket.Find(bson.M{"metadata.short_id": fileID})
		if err != nil {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		defer cursor.Close(ctx)

		var fileDoc struct {
			Filename string `bson:"filename"`
			Length   int64  `bson:"length"`
			Metadata struct {
				ContentType string `bson:"content_type"`
			} `bson:"metadata"`
		}

		if !cursor.Next(ctx) {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}

		err = cursor.Decode(&fileDoc)
		if err != nil {
			http.Error(w, "decode error", http.StatusInternalServerError)
			return
		}

		fileType := getFileType(fileDoc.Metadata.ContentType)

		data := struct {
			FileID   string
			Filename string
			FileSize string
		}{
			FileID:   fileID,
			Filename: fileDoc.Filename,
			FileSize: formatSize(fileDoc.Length),
		}

		var tmpl *template.Template
		switch fileType {
		case "image":
			tmpl = template.Must(template.ParseFiles("templates/viewer_image.html"))
		case "video":
			tmpl = template.Must(template.ParseFiles("templates/viewer_video.html"))
		case "audio":
			tmpl = template.Must(template.ParseFiles("templates/viewer_audio.html"))
		default:
			tmpl = template.Must(template.ParseFiles("templates/viewer_file.html"))
		}

		tmpl.Execute(w, data)
	})

	http.HandleFunc("/integrations", func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.ParseFiles("templates/integrations.html"))
		tmpl.Execute(w, nil)
	})

	http.HandleFunc("/deployment", func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.ParseFiles("templates/deployment.html"))
		tmpl.Execute(w, nil)
	})

	http.HandleFunc("/raw/", func(w http.ResponseWriter, r *http.Request) {
		fileID := r.URL.Path[len("/raw/"):]
		if fileID == "" {
			http.Error(w, "no file id", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cursor, err := gfsBucket.Find(bson.M{"metadata.short_id": fileID})
		if err != nil {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		defer cursor.Close(ctx)

		var fileDoc struct {
			ID       interface{} `bson:"_id"`
			Filename string      `bson:"filename"`
			Metadata struct {
				ContentType string `bson:"content_type"`
			} `bson:"metadata"`
		}

		if !cursor.Next(ctx) {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}

		err = cursor.Decode(&fileDoc)
		if err != nil {
			http.Error(w, "decode error", http.StatusInternalServerError)
			return
		}

		downloadStream, err := gfsBucket.OpenDownloadStream(fileDoc.ID)
		if err != nil {
			http.Error(w, "download error", http.StatusInternalServerError)
			return
		}
		defer downloadStream.Close()

		w.Header().Set("Content-Type", fileDoc.Metadata.ContentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileDoc.Filename))
		io.Copy(w, downloadStream)
	})

	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		err := r.ParseMultipartForm(config.Upload.MaxSize)
		if err != nil {
			jsonError(w, "Bad request", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			jsonError(w, "File not found", http.StatusBadRequest)
			return
		}
		defer file.Close()

		if header.Size > config.Upload.MaxSize {
			jsonError(w, fmt.Sprintf("File too large (max %d MB)", config.Upload.MaxSize/(1024*1024)), http.StatusBadRequest)
			return
		}

		shortID := generateID()
		deleteToken := generateID() + generateID()
		contentType := header.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		opts := options.GridFSUpload().SetMetadata(bson.M{
			"short_id":     shortID,
			"delete_token": deleteToken,
			"content_type": contentType,
		})

		uploadStream, err := gfsBucket.OpenUploadStream(header.Filename, opts)
		if err != nil {
			jsonError(w, "Upload error", http.StatusInternalServerError)
			return
		}
		defer uploadStream.Close()

		_, err = io.Copy(uploadStream, file)
		if err != nil {
			jsonError(w, "Write error", http.StatusInternalServerError)
			return
		}

		response := map[string]string{
			"link":          fmt.Sprintf("%s/%s", config.Upload.BaseURL, shortID),
			"deletion_link": fmt.Sprintf("%s/delete/%s", config.Upload.BaseURL, deleteToken),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	http.HandleFunc("/delete/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		deleteToken := r.URL.Path[len("/delete/"):]
		if deleteToken == "" {
			jsonError(w, "No delete token", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cursor, err := gfsBucket.Find(bson.M{"metadata.delete_token": deleteToken})
		if err != nil {
			jsonError(w, "File not found", http.StatusNotFound)
			return
		}
		defer cursor.Close(ctx)

		var fileDoc struct {
			ID interface{} `bson:"_id"`
		}

		if !cursor.Next(ctx) {
			jsonError(w, "File not found", http.StatusNotFound)
			return
		}

		err = cursor.Decode(&fileDoc)
		if err != nil {
			jsonError(w, "Decode error", http.StatusInternalServerError)
			return
		}

		err = gfsBucket.Delete(fileDoc.ID)
		if err != nil {
			jsonError(w, "Delete error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
	})

	addr := fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port)
	log.Printf("Starting server on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
