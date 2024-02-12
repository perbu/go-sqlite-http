# Non-OO servers in Go


## Typical Go server

```go

package main

import (
	"fmt"
	"golang.org/x/tools/go/types/typeutil"
	"net/http"
)

type Server struct {
	dependency database.Connection
}

func NewServer(dep database.Connection) *Server {
	return &Server{dependency: dep}
}

func (s *Server) RootHandler(w http.ResponseWriter, r *http.Request) {
	// do something with s.dependency:
	s.dependency.Query("SELECT * FROM users")
	fmt.Fprintf(w, "Hello, Users!")
}

func main() {
	dep := database.NewConnection()
	s := NewServer(dep)
	http.HandleFunc("/", s.RootHandler)
	http.ListenAndServe(":8080", nil)
}

```


