package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/DarkLordOfDeadstiny/DSYS-gRPC-template/proto"
	gRPC "github.com/DarkLordOfDeadstiny/DSYS-gRPC-template/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Same principle as in client. Flags allows for user specific arguments/values
var clientsName = flag.String("name", "default", "Senders name")
var serverPort = flag.String("server", "5400", "Tcp server")
var server gRPC.ChittyChatClient //the server
var ServerConn *grpc.ClientConn  //the server connection
var LamportClock int32

func main() {
	//parse flag/arguments
	flag.Parse()

	fmt.Println("--- CLIENT APP ---")

	//log to file instead of console
	//setLog()

	//connect to server and close the connection when program closes
	fmt.Println("--- join Server ---")
	ConnectToServer()
	defer ServerConn.Close()

	ctx := context.Background()
	client := proto.NewChittyChatClient(ServerConn)

	go joinChat(ctx, client)

	parseInput(ctx, server)
}

func DisconnectFromServer() {
	ctx := context.Background()
	server := proto.NewChittyChatClient(ServerConn)
	sendMessage(ctx, server, "close")
	log.Printf("Closing connection to server from %v", *clientsName)
	LamportClock = 0
}

// connect to server
func ConnectToServer() {

	//dial options
	//the server is not using TLS, so we use insecure credentials
	//(should be fine for local testing but not in the real world)
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithBlock(), grpc.WithTransportCredentials(insecure.NewCredentials()))

	//use context for timeout on the connection
	timeContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel() //cancel the connection when we are done

	//dial the server to get a connection to it
	log.Printf("client %s: Attempts to dial on port %s\n", *clientsName, *serverPort)
	// Insert your device's IP before the colon in the print statement
	conn, err := grpc.DialContext(timeContext, fmt.Sprintf("localhost:%s", *serverPort), opts...)
	if err != nil {
		log.Printf("Fail to Dial : %v", err)
		return
	}

	// makes a client from the server connection and saves the connection
	// and prints rather or not the connection was is READY
	server = gRPC.NewChittyChatClient(conn)
	ServerConn = conn
	log.Println("the connection is: ", conn.GetState().String())
}

func parseInput(ctx context.Context, server gRPC.ChittyChatClient) {

	reader := bufio.NewReaderSize(os.Stdin, 128)

	//Infinite loop to listen for clients input.
	for {
		//Read input into var input and any errors into err
		input, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}
		input = strings.TrimSpace(input) //Trim input
		if len(input) > 128 {
			fmt.Println("Message must be shorter than 128 characters. Please try again...")
			continue
		}
		if input == "close" {
			DisconnectFromServer()
			break
		}
		if !conReady(server) {
			log.Printf("Client %s: something was wrong with the connection to the server :(", *clientsName)
			continue
		}
		sendMessage(ctx, server, input)

		continue
	}
}

func joinChat(ctx context.Context, server proto.ChittyChatClient) {
	ack := proto.Message{Sender: *clientsName}
	stream, err := server.JoinChat(ctx, &ack)
	if err != nil {
		log.Fatalf("client.JoinChat(ctx, &channel) throws: %v", err)
	}
	fmt.Printf("Joined server: %v \n", *clientsName)
	sendMessage(ctx, server, "join")
	waitc := make(chan struct{})

	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				fmt.Println("not working")
				close(waitc)
				return
			}
			if err != nil {
				log.Fatalf("Failed to receive message from channel joining. \nErr: %v", err)
			}
			if in.Lamport > LamportClock {
				LamportClock = in.Lamport
			}
			LamportClock++
			fmt.Printf("Lamport: %v, Message from %v: %v \n", LamportClock, in.Sender, in.Message)
			fmt.Println("--------------------")
		}
	}()

	<-waitc
}

func sendMessage(ctx context.Context, server proto.ChittyChatClient, message string) {
	stream, err := server.SendMessage(ctx)
	if err != nil {
		log.Printf("Cannot send message: error: %v", err)
	}
	LamportClock++
	msg := proto.Message{
		Message: message,
		Sender:  *clientsName,
		Lamport: LamportClock,
	}
	stream.Send(&msg)

	ack, err := stream.CloseAndRecv()
	fmt.Printf("Message status: %v \n", ack.Status)
}

// Function which returns a true boolean if the connection to the server is ready, and false if it's not.
func conReady(s gRPC.ChittyChatClient) bool {
	return ServerConn.GetState().String() == "READY"
}

// sets the logger to use a log.txt file instead of the console
func setLog() {
	f, err := os.OpenFile("log.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)
}
