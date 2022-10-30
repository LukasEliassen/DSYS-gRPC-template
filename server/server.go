package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"time"

	// this has to be the same as the go.mod module,
	// followed by the path to the folder the proto file is in.
	"github.com/DarkLordOfDeadstiny/DSYS-gRPC-template/proto"
	gRPC "github.com/DarkLordOfDeadstiny/DSYS-gRPC-template/proto"

	"google.golang.org/grpc"
)

type Server struct {
	gRPC.UnimplementedChatServiceServer        // You need this line if you have a server
	name                                string // Not required but useful if you want to name your server
	port                                string // Not required but useful if your server needs to know what port it's listening to
	channel                             map[string]chan *proto.Message
	lamportClock                        int32
}

// flags are used to get arguments from the terminal. Flags take a value, a default value and a description of the flag.
// to use a flag then just add it as an argument when running the program.
var serverName = flag.String("name", "default", "Senders name") // set with "-name <name>" in terminal
var port = flag.String("port", "5400", "Server port")           // set with "-port <port>" in terminal

func main() {
	// This parses the flags and sets the correct/given corresponding values.
	flag.Parse()
	fmt.Println("Server is starting...")

	// starts a goroutine executing the launchServer method.
	go launchServer()

	// This makes sure that the main method is "kept alive"/keeps running
	for {
		time.Sleep(time.Second * 5)
	}
}

func launchServer() {
	log.Printf("Server %s: Attempts to create listener on port %s\n", *serverName, *port)

	// Create listener tcp on given port or default port 5400
	// Insert your device's IP before the colon in the print statement
	list, err := net.Listen("tcp", "localhost:5400")
	if err != nil {
		log.Printf("Server %s: Failed to listen on port %s: %v", *serverName, *port, err) //If it fails to listen on the port, run launchServer method again with the next value/port in ports array
		return
	}

	// makes gRPC server using the options
	// you can add options here if you want or remove the options part entirely
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)

	// makes a new server instance using the name and port from the flags.
	server := &Server{
		name:         *serverName,
		port:         *port,
		lamportClock: 0,
		channel:      make(map[string]chan *proto.Message),
	}

	gRPC.RegisterChatServiceServer(grpcServer, server) //Registers the server to the gRPC server

	log.Printf("Server %s: Listening on port %s\n", *serverName, *port)

	if err := grpcServer.Serve(list); err != nil {
		log.Fatalf("failed to serve %v", err)
	}
	// code here is unreachable because grpcServer.Serve occupies the current thread.
}

func (s *Server) JoinChannel(msg *proto.Message, msgStream proto.ChatService_JoinChannelServer) error {

	msgChannel := make(chan *proto.Message)
	s.channel[msg.Sender] = msgChannel
	for {
		select {
		case <-msgStream.Context().Done():
			return nil
		case msg := <-msgChannel:
			s.lamportClock++
			msg.Lamport = s.lamportClock
			msgStream.Send(msg)
		}
	}
}

func (s *Server) SendMessage(msgStream proto.ChatService_SendMessageServer) error {
	msg, err := msgStream.Recv()

	if err == io.EOF {
		return nil
	}

	if err != nil {
		return err
	}
	if msg.Lamport > s.lamportClock {
		s.lamportClock = msg.Lamport
	}
	log.Printf("Received message: %v \n", msg)
	s.lamportClock++
	if msg.Message == "close" {
		delete(s.channel, msg.Sender)
		log.Printf("Closing connection to client %v", msg.Sender)
		ack := proto.MessageAck{Status: "DISCONNECTED"}
		msgStream.SendAndClose(&ack)
		msg.Message = "Participant " + msg.Sender + " has left ChittyChat at Lamport time " + strconv.Itoa(int(s.lamportClock))
	} else if msg.Message == "join" {
		log.Printf("Participant %v has joined ChittyChat at Lamport time "+strconv.Itoa(int(s.lamportClock)), msg.Sender)
		msg.Message = "Participant " + msg.Sender + " has joined ChittyChat at Lamport time " + strconv.Itoa(int(s.lamportClock))
		ack := proto.MessageAck{Status: "CONNECTED"}
		msgStream.SendAndClose(&ack)
	} else {
		ack := proto.MessageAck{Status: "SENT"}
		msgStream.SendAndClose(&ack)
	}
	go func() {
		for client, msgChan := range s.channel {
			if client != msg.Sender {
				msgChan <- msg
			}
		}
	}()

	return nil
}
