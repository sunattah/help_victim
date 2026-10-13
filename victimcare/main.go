
package main

import (
	"log"
	"net/http"
)

func main() {
	// Serve the Victim Care homepage.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		http.ServeFile(w, r, "templates/index.html")
	})

	// Start the web server.
	log.Println("Victim Care is running at http://localhost:8080")

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}
