package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

//OfferDescription object
type OfferDescription struct {
	Offer            string `json:"offer"`
	IngestionAddress string `json:"ingestionAddress"`
	StreamKey        string `json:"streamKey"`
}

func setupResponse(w *http.ResponseWriter, req *http.Request) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

func createWebRTCOffer(w http.ResponseWriter, r *http.Request) {
	setupResponse(&w, r)

	if r.URL.Path != "/webrtc/offer" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if r.Method == "OPTIONS" {
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method is not supported.", http.StatusNotFound)
		return
	}

	offerDesc := OfferDescription{}

	err := DecodeJSONBody(w, r, &offerDesc)
	if err != nil {
		var mr *malformedRequest
		if errors.As(err, &mr) {
			http.Error(w, mr.msg, mr.status)
		} else {
			log.Println(err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	offer, err := CreateWebRTCConnection(offerDesc.IngestionAddress, offerDesc.StreamKey, offerDesc.Offer)

	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(offer)
}

//InitializeRoutes exported function to intiailize routes
func InitializeRoutes() {
	http.HandleFunc("/webrtc/offer", createWebRTCOffer)
}
