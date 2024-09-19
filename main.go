package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	terminatorString = "</event>"
	bufferSize       = 65536
)

type Server struct {
	addr       *net.UDPAddr
	conn       *net.UDPConn
	clients    map[string]*net.UDPAddr
	messageBuf string
}

func NewServer(port string) (*Server, error) {
	addr, err := net.ResolveUDPAddr("udp", ":"+port)
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}

	server := &Server{
		addr:    addr,
		conn:    conn,
		clients: make(map[string]*net.UDPAddr),
	}

	return server, nil
}

func (s *Server) Start() {
	defer s.conn.Close()

	fmt.Println("Press 'Q' and 'Enter' to exit the program.")

	// Channel to handle exit signal
	exitChan := make(chan os.Signal, 1)
	signal.Notify(exitChan, os.Interrupt, syscall.SIGTERM)

	// Channel to read keyboard input
	inputChan := make(chan string)
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			input, _ := reader.ReadString('\n')
			inputChan <- strings.TrimSpace(input)
		}
	}()

	buf := make([]byte, bufferSize)

	for {
		select {
		case <-exitChan:
			fmt.Println("\nServer shutting down.")
			return
		case input := <-inputChan:
			if strings.EqualFold(input, "Q") {
				fmt.Println("Exit command received. Shutting down server.")
				return
			}
			fmt.Printf("%d clients currently connected. Press 'Q' and 'Enter' to exit the program.\n", len(s.clients))
		default:
			s.conn.SetReadDeadline(getTimeout())
			n, addr, err := s.conn.ReadFromUDP(buf)
			if err != nil {
				if isTimeout(err) {
					continue
				}
				fmt.Println("Error reading from UDP:", err)
				continue
			}

			s.addClient(addr)
			s.parseData(buf[:n])
		}
	}
}

func (s *Server) addClient(addr *net.UDPAddr) {
	key := addr.String()
	if _, exists := s.clients[key]; !exists {
		s.clients[key] = addr
		fmt.Printf("New client added: %s\n", key)
	}
}

func (s *Server) parseData(data []byte) {
	s.messageBuf += string(data)
	for {
		idx := strings.Index(s.messageBuf, terminatorString)
		if idx == -1 {
			break
		}
		// Extract complete message including the terminator
		completeMessage := s.messageBuf[:idx+len(terminatorString)]
		s.shareData([]byte(completeMessage))
		// Remove the processed message from the buffer
		s.messageBuf = s.messageBuf[idx+len(terminatorString):]
	}
}

func (s *Server) shareData(data []byte) {
	for key, addr := range s.clients {
		_, err := s.conn.WriteToUDP(data, addr)
		if err != nil {
			fmt.Printf("Error sending to client %s: %v. Removing client.\n", key, err)
			delete(s.clients, key)
		}
	}
}

func getTimeout() time.Time {
	return time.Now().Add(100 * time.Millisecond)
}

func isTimeout(err error) bool {
	nErr, ok := err.(net.Error)
	return ok && nErr.Timeout()
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s <port>\n", os.Args[0])
		return
	}

	port := os.Args[1]
	server, err := NewServer(port)
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}

	server.Start()
}
