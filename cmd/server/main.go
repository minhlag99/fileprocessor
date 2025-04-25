// Package main is the entry point for the file processor server
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-fileprocessor/internal/handlers"
	"go-fileprocessor/internal/processors"
)

var (
	port            = flag.Int("port", 8080, "Server port")
	uiDir           = flag.String("ui", "./ui", "UI directory")
	uploads         = flag.String("uploads", "./uploads", "Upload directory")
	certFile        = flag.String("cert", "", "TLS certificate file (optional)")
	keyFile         = flag.String("key", "", "TLS key file (optional)")
	workerCount     = flag.Int("workers", runtime.NumCPU(), "Number of worker threads for file processing")
	enableLAN       = flag.Bool("enable-lan", true, "Enable LAN file transfer capabilities")
	shutdownTimeout = flag.Int("shutdown-timeout", 30, "Graceful shutdown timeout in seconds")
	version         = "1.0.0"  // Version number for the application
)

func main() {
	// Parse command line flags
	flag.Parse()

	// Print banner and version
	fmt.Printf("\n=================================\n")
	fmt.Printf("File Management System v%s\n", version)
	fmt.Printf("=================================\n\n")
	fmt.Printf("Server starting with %d worker threads\n", *workerCount)
	if *enableLAN {
		fmt.Printf("LAN file transfer capabilities enabled\n")
	}

	// Create directories if they don't exist
	for _, dir := range []string{*uploads, *uiDir} {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				log.Fatalf("Failed to create directory %s: %v", dir, err)
			}
		}
	}

	// Initialize worker pool with specified number of workers
	log.Printf("Initializing worker pool with %d workers", *workerCount)
	processors.DefaultPool = processors.NewWorkerPool(*workerCount)

	// Initialize file handler
	fileHandler, err := handlers.NewFileHandler()
	if err != nil {
		log.Fatalf("Failed to initialize file handler: %v", err)
	}

	// Initialize LAN transfer handler if enabled
	var lanHandler *handlers.LANTransferHandler
	if *enableLAN {
		log.Println("Initializing LAN file transfer capabilities")
		lanHandler, err = handlers.NewLANTransferHandler()
		if err != nil {
			log.Printf("Failed to initialize LAN transfer handler: %v", err)
			log.Println("LAN file transfer will be disabled")
		} else {
			if err := lanHandler.Start(); err != nil {
				log.Printf("Failed to start LAN transfer service: %v", err)
				log.Println("LAN file transfer will be disabled")
				lanHandler = nil
			}
		}
	}

	// Create router and register handlers
	mux := http.NewServeMux()

	// File API routes
	mux.HandleFunc("/api/upload", fileHandler.UploadFile)
	mux.HandleFunc("/api/download", fileHandler.DownloadFile)
	mux.HandleFunc("/api/list", fileHandler.ListFiles)
	mux.HandleFunc("/api/url", fileHandler.GetSignedURL)
	mux.HandleFunc("/api/delete", fileHandler.DeleteFile)
	mux.HandleFunc("/api/storage/status", fileHandler.GetStorageProviderStatus)

	// WebSocket endpoint for real-time updates
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handlers.ServeWs(handlers.DefaultWebSocketHub, w, r)
	})

	// LAN transfer routes if enabled
	if lanHandler != nil {
		mux.HandleFunc("/api/lan/discover", lanHandler.HandleDiscoverPeers)
		mux.HandleFunc("/api/lan/initiate", lanHandler.HandleInitiateTransfer)
		mux.HandleFunc("/api/lan/accept", lanHandler.HandleAcceptTransfer)
		mux.HandleFunc("/api/lan/status", lanHandler.HandleTransferStatus)
	}

	// Special handler for workflow.gif
	mux.HandleFunc("/workflow.gif", func(w http.ResponseWriter, r *http.Request) {
		// For demonstration purposes, we'll serve the workflow HTML page 
		// which animates the workflow process
		w.Header().Set("Content-Type", "text/html")
		http.ServeFile(w, r, filepath.Join(*uiDir, "images/workflow.html"))
	})

	// Serve static UI files
	mux.Handle("/", http.FileServer(http.Dir(*uiDir)))

	// Create HTTP server
	addr := fmt.Sprintf(":%d", *port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Channel to listen for interrupt signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start the server in a goroutine
	go func() {
		log.Printf("Starting server on %s", addr)

		var err error
		if *certFile != "" && *keyFile != "" {
			// HTTPS server
			log.Printf("Using TLS with cert file %s and key file %s", *certFile, *keyFile)
			err = server.ListenAndServeTLS(*certFile, *keyFile)
		} else {
			// HTTP server
			err = server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-stop
	log.Println("Shutting down server...")

	// Create context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*shutdownTimeout)*time.Second)
	defer cancel()

	// Shutdown the server gracefully
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// Stop the worker pool
	processors.DefaultPool.Stop()
	log.Println("Worker pool stopped")

	// Stop LAN transfer service if it was enabled
	if lanHandler != nil {
		lanHandler.Stop()
	}

	log.Println("Server shutdown complete")
}