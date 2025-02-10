package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"dotnetdev/internal/auth"

	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	const maxMemory = 10 << 20 // 10 MB
	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	mediaType := header.Header.Get("Content-Type")
	if mediaType == "" {
		respondWithError(w, http.StatusBadRequest, "Missing Content-Type for thumbnail", nil)
		return
	}
	if !strings.HasPrefix(mediaType, "image/") {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type for thumbnail", nil)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	fmt.Printf("Initial video: %+v\n", video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't find video", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorized to update this video", nil)
		return
	}

	mediaExtension := mediaType[strings.LastIndex(mediaType, "/")+1:]
	randomBytes := make([]byte, 32)
	_, err = rand.Read(randomBytes)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error generating random bytes", err)
		return
	}
	encodedRand := base64.RawURLEncoding.EncodeToString(randomBytes)

	path := filepath.Join(cfg.assetsRoot, fmt.Sprintf("%v.%v", encodedRand, mediaExtension))

	thumbnail, err := os.Create(path)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating file", err)
		return
	}
	defer thumbnail.Close()

	if _, err := io.Copy(thumbnail, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error writing to file", err)
		return
	}

	baseURL := fmt.Sprintf("http://localhost:%v", cfg.port)

	thumbnailURL := fmt.Sprintf("%s/assets/%v.%v", baseURL, encodedRand, mediaExtension)

	video.ThumbnailURL = &thumbnailURL

	err = cfg.db.UpdateVideo(video)
	fmt.Printf("Update error: %v\n", err)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}
	fmt.Printf("Final video state: %+v\n", video)
	respondWithJSON(w, http.StatusOK, video)
}
