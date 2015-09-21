package main

import "bufio"
import "fmt"
import "io"
import "log"
import "net"
import "strconv"
import "strings"
import "time"
import c "github.com/fimmtiu/gofingerd/config"
import r "github.com/fimmtiu/gofingerd/request"
import u "github.com/fimmtiu/gofingerd/users"

var conf c.Config
var userService chan u.UserCommand

func main() {
	conf = c.ReadConfig()

	// FIXME: It would be nice to provide an option to daemonize the process here.

	// Open the listening socket
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(conf.Port))
	if listener == nil {
		log.Fatalf("ERROR: Can't listen on port %d: %s", conf.Port, err.Error())
	}
	defer listener.Close()
	log.Printf("Listening on port %d.", conf.Port)

	// Spin up the user service goroutine. (See comments in users.go.)
	userService = make(chan u.UserCommand, 1)
	go u.StartService(userService)

	// For every incoming connection, spin up a goroutine to handle the request.
	for {
		conn, err := listener.Accept()
		if err == nil {
			go handleConnection(conn)
		} else {
			log.Fatalf("ERROR: Couldn't accept connection: %s", err.Error())
		}
	}
}

// Read user input and send a response on a single client connection.
func handleConnection(conn net.Conn) {
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(conf.NetworkTimeout))
	request, err := readRequest(conn)
	if err != nil {
		log.Printf("ERROR: Can't read from %s: %s", conn.RemoteAddr().String(), err.Error())
		return
	}

	if len(request.Hosts) > 0 {
		if conf.AllowQueryForwarding {
			RespondWithForwardedQuery(conn, request)
		} else {
			log.Printf("Refused forwarding request from %s.", conn.RemoteAddr().String())
			fmt.Fprint(conn, "This server does not permit request forwarding.\r\n")
		}
	} else if len(request.User) == 0 {
		if conf.AllowUserListing {
			RespondWithUserList(conn)
		} else {
			log.Printf("Refused user listing request from %s.", conn.RemoteAddr().String())
			fmt.Fprint(conn, "This server does not permit user listing.\r\n")
		}
	} else if conf.AllowApproximateSearch {
		RespondWithApproximateSearch(conn, request)
	} else {
		RespondWithExactSearch(conn, request)
	}
}

// Parse a line of user input into a Request object.
func readRequest(conn net.Conn) (r.Request, error) {
	input, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return r.ParseRequest(""), err
	}
	input = strings.TrimSuffix(input, "\r\n")
	input = strings.TrimSuffix(input, "\n")
	log.Printf("Received request from %s: \"%s\"", conn.RemoteAddr().String(), input)
	return r.ParseRequest(input), nil
}

// Connect to a remote server, forward the user's query to them, and echo the response.
func RespondWithForwardedQuery(conn net.Conn, request r.Request) {
	host, forwarded_request := request.NextForwardingRequest()
	forward_conn, err := net.Dial("tcp", host+":79")
	if err != nil {
		log.Printf("Can't connect to %s on behalf of %s: %s.", host, conn.RemoteAddr().String(), err.Error())
		fmt.Fprintf(conn, "Can't connect to %s: %s.\r\n", host, err.Error())
		return
	}
	defer forward_conn.Close()
	fmt.Fprintf(forward_conn, "%s\r\n", forwarded_request)

	var bytes []byte
	reader := bufio.NewReader(forward_conn)
	for {
		_, err := reader.Read(bytes)
		if err != nil && err != io.EOF {
			log.Printf("Error reading from %s on behalf of %s: %s.", host, conn.RemoteAddr().String(), err.Error())
			fmt.Fprintf(conn, "Error reading from %s: %s.\r\n", host, err.Error())
			return
		}
		if _, write_err := conn.Write(bytes); write_err != nil {
			log.Printf("Error writing forwarded response from %s to %s: %s.", host, conn.RemoteAddr().String(), err.Error())
			return
		}
		if err == io.EOF {
			break
		}
	}
}

// Show a list of all users on the system.
func RespondWithUserList(conn net.Conn) {
	responseChan := make(chan u.UserCommand, 1)
	cmd := u.UserCommand{u.ListAll, responseChan, "", nil, nil}
	userService <- cmd
	cmd = <-responseChan

	if cmd.Err != nil {
		log.Printf("ERROR: Can't enumerate users for %s: %s.", conn.RemoteAddr().String(), cmd.Err.Error())
		fmt.Fprint(conn, "An error occurred. Please try again later.\r\n")
		return
	}

	fmt.Fprint(conn, "User Name         Full Name\r\n")
	for _, user := range cmd.Users {
		fmt.Fprintf(conn, "%-16s  %s\r\n", user.LoginName, user.FullName)
	}
}

// Show all users whose name contains the client's input.
func RespondWithApproximateSearch(conn net.Conn, request r.Request) {
	responseChan := make(chan u.UserCommand, 1)
	cmd := u.UserCommand{u.Search, responseChan, request.User, nil, nil}
	userService <- cmd
	cmd = <-responseChan

	if cmd.Err != nil {
		fmt.Fprint(conn, "An error occurred. Please try again later.\r\n")
		log.Printf("ERROR: Can't enumerate users for %s: %s", conn.RemoteAddr().String(), cmd.Err.Error())
		return
	}

	if len(cmd.Users) == 0 {
		log.Printf("Found no users matching query \"%s\" from %s.", request.User, conn.RemoteAddr().String())
		fmt.Fprint(conn, "Found no users matching your query.\r\n")
	} else {
		log.Printf("Found %d users matching query \"%s\" from %s.", len(cmd.Users), request.User, conn.RemoteAddr().String())
		for _, user := range cmd.Users {
			RespondWithUser(conn, user)
		}
	}
}

// Show the user whose exact name was provided by the client.
func RespondWithExactSearch(conn net.Conn, request r.Request) {
	responseChan := make(chan u.UserCommand, 1)
	cmd := u.UserCommand{u.Find, responseChan, request.User, nil, nil}
	userService <- cmd
	cmd = <-responseChan

	if cmd.Err != nil {
		log.Printf("ERROR: Can't look up user \"%s\" for %s: %s", request.User, conn.RemoteAddr().String(), cmd.Err.Error())
		fmt.Fprint(conn, "An error occurred. Please try again later.\r\n")
		return
	}

	if len(cmd.Users) > 0 {
		RespondWithUser(conn, cmd.Users[0])
	} else {
		log.Printf("Found no user matching query \"%s\" for %s.", request.User, conn.RemoteAddr().String())
		fmt.Fprint(conn, "Found no users matching your query.\r\n")
	}
}

// Show a detailed description of an individual user.
func RespondWithUser(conn net.Conn, user u.User) {
	fmt.Fprintf(conn, "Login name: %-30s Home directory: %s\r\n", user.LoginName, user.HomeDir)
	fmt.Fprintf(conn, "Full name: %-31s Shell: %s\r\n", user.FullName, user.Shell)
	conn.Write(user.GetPlanFile())
	fmt.Fprint(conn, "\r\n")
}
