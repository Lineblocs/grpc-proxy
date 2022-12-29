package main

import (
	"strings"
	"net"
	"log"
	"fmt"
	"net/http"
	"net/url"
	"database/sql"
	"github.com/mwitkow/grpc-proxy/proxy"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
		"github.com/koding/websocketproxy"
	lineblocs "github.com/Lineblocs/go-helpers"
)

var (
	director proxy.StreamDirector
)
var db* sql.DB;

func healthz(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "text/plain")
  fmt.Fprintf(w, "OK\n")
}
func getAvailableMediaserver() (*lineblocs.MediaServer, error) {
	servers, err := lineblocs.CreateMediaServers()
	if err != nil {
		return nil,err
	}

	return servers[0], nil
}
func proxyWebsocket() {
	server, err := getAvailableMediaserver()
	if err != nil {
		log.Fatalln(err)
	}
	splitted := strings.Split(server.PrivateIpAddress,":")
	addr:= "ws://" + splitted[0] + ":8018"
	fmt.Println("WS addr: " + addr)
	backend, err := url.Parse(addr)
	if err != nil {
		log.Fatalln(err)
	}

	/*
	getWSAddr := func() *url.URL {
		return nil
	}
	*/

	http.HandleFunc("/healthz", healthz)
	//err = http.ListenAndServe(":8017", websocketproxy.NewProxy(backend, getWSAddr))
	err = http.ListenAndServe(":8017", websocketproxy.NewProxy(backend))
	if err != nil {
		log.Fatalln(err)
	}
}
func LineblocsTransparentHandler() {
	grpc.NewServer(
		grpc.CustomCodec(proxy.Codec()),
		grpc.UnknownServiceHandler(proxy.TransparentHandler(LineblocsStreamDirector)))
}

// Provide sa simple example of a director that shields internal services and dials a staging or production backend.
// This is a *very naive* implementation that creates a new connection on every request. Consider using pooling.
func LineblocsStreamDirector(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, error) {
	// Make sure we never forward internal services.
	fmt.Println("method is: " + fullMethodName)
	if strings.HasPrefix(fullMethodName, "/com.example.internal.") {
		return nil, nil, grpc.Errorf(codes.Unimplemented, "Unknown method")
	}
	md, ok := metadata.FromIncomingContext(ctx)
	// Copy the inbound metadata explicitly.
	outCtx, _ := context.WithCancel(ctx)
	outCtx = metadata.NewOutgoingContext(outCtx, md.Copy())
	fmt.Println("created out context...")
	if ok {
		// Decide on which backend to dial
		fmt.Println("ROUTING NOW...")
		val, exists := md[":authority"]
		if !exists {
			fmt.Println("no authority found..\r\n");
			return nil, nil, grpc.Errorf(codes.Unimplemented, "no authority found")

		}
		authority := val[0]
		fmt.Println("authority IP: " + authority)

		server, err := getAvailableMediaserver()
		if err != nil {
			return nil, nil, err
		}
		splitted := strings.Split(server.PrivateIpAddress,":")
		ip:= splitted[0]+":9000"

		fmt.Println("routing to server: " + ip)
		conn, err := grpc.DialContext(ctx, ip, grpc.WithCodec(proxy.Codec()), grpc.WithInsecure())
		return outCtx, conn, err
	}
	return nil, nil, grpc.Errorf(codes.Unimplemented, "Unknown method")
}

func main() {
	var err error
	fmt.Println("starting server..")
	db, err =lineblocs.CreateDBConn()
	if err != nil {
		panic(err)
	}
	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", 9001))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	fmt.Println("started listener")
	//grpcServer := grpc.NewServer()
	//grpcServer := grpc.NewServer(grpc.CustomCodec(proxy.Codec()))
	director := LineblocsStreamDirector
	grpcServer := grpc.NewServer(grpc.CustomCodec(proxy.Codec()))
	proxy.RegisterService(grpcServer, director,
		"grpc.Lineblocs",
"createBridge",
"createCall",
"addChannel",
"playRecording",
"getChannel",
"createConference",
"channel_getBridge",
"channel_removeFromBridge",
"channel_playTTS",
"channel_startAcceptingInput",
"channel_removeDTMFListeners",
"channel_automateCallHangup",
"channel_gotoFlowWidget",
"channel_startFlow",
"channel_startRinging",
"channel_stopRinging",
"channel_record",
"channel_hangup",
"bridge_addChannel",
"bridge_addChannels",
"bridge_removeChannel",
"bridge_playTTS",
"bridge_automateLegAHangup",
"bridge_automateLegBHangup",
"bridge_hangupChannel",
"bridge_hangupAllChannels",
"bridge_getChannels",
"bridge_destroy",
"bridge_record",
"bridge_attachEventListener",
"conference_addWaitingParticipant",
"conference_addParticipant",
"conference_setModeratorInConf",
"conference_attachEventListener",
"recording_stop")
 	//LineblocsRegisterService(grpcServer)
	//RegisterLineblocsServer(grpcServer, s)

	go proxyWebsocket()
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %s", err)
	}
}