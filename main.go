package main

import (
    "fmt"
    "net/http"
    "github.com/gorilla/websocket"
    "os"
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
    "encoding/json"
    "time"
    "strconv"
)

var db *sql.DB
var err error

type Message struct {
    mt int
    message []byte
}

var msgChans [](chan Message)

func serveHome(w http.ResponseWriter, r *http.Request) {
        _, err := r.Cookie("chatter_user_name")
        if err != nil {
            http.Redirect(w, r, "/login", 301)
        } else {
            http.ServeFile(w, r, "home.html")
        }
}

type MessageStruct struct {
    UserFrom string `json:"userFrom"`
    UserTo string   `json:"userTo"`
    Content string  `json:"content"`
    Date string `json:"date"`
    Time string `json:"time"`
    Type int `json:"type"`
}

func serveWS(w http.ResponseWriter, r *http.Request) {
    upgrader := websocket.Upgrader {
        ReadBufferSize : 1024,
        WriteBufferSize: 1024,
    }

    conn, err := upgrader.Upgrade(w, r, nil)
    checkError(err, "upgrader.Upgrade")
    defer conn.Close()

    username_cookie, err := r.Cookie("chatter_user_name")
    checkError(err, "[1] r.Cookie")
    user_name := username_cookie.Value

    var user_id uint64
    err = db.QueryRow("select user_id from users where user_name = ?", user_name).Scan(&user_id)
    checkError(err, "db.QueryRow(user_id)")

    // list streams
    stream_names := []string { "1" }
    rows, err := db.Query("select stream_name from streams")
    checkError(err, "db.Query")
    defer rows.Close()
    for rows.Next() {
        var stream_name string
        err := rows.Scan(&stream_name)
        checkError(err, "rows.Scan")
        stream_names = append(stream_names, stream_name)
    }
    // conn.WriteMessage(json.Marshal(stream_names))
    conn.WriteJSON(stream_names)

    // list subscribed streams
    subscribed_streams := []string { "6" }
    rows, err = db.Query("select stream_id from stream_subscribers where user_id = ?", user_id)
    checkError(err, "db.Query(stream_id)")
    defer rows.Close()
    for rows.Next() {
        var stream_id int64
        err := rows.Scan(&stream_id)
        checkError(err, "rows.Scan(&stream_id)")

        var stream_name string
        err = db.QueryRow("select stream_name from streams where stream_id = ?", stream_id).Scan(&stream_name)
        subscribed_streams = append(subscribed_streams, stream_name)
    }
    conn.WriteJSON(subscribed_streams)

    // list users
    user_names := []string { "3" }
    name_rows, err := db.Query("select user_name from users")
    checkError(err, "db.Query")
    defer name_rows.Close()
    for name_rows.Next() {
        var n string
        err := name_rows.Scan(&n)
        fmt.Println("find name:", n)
        checkError(err, "name_rows.Scan")
        user_names = append(user_names, n)
    }
    conn.WriteJSON(user_names)

    // 4 list user's messages
    messages := []interface {} { }
    messages = append(messages, "4")
    mr, err := db.Query("select user_from, user_to, message, message_date, message_time, message_type from messages where user_to=?", user_name)
    checkError(err, "db.Query")
    defer mr.Close()
    for mr.Next() {
        var user_from string
        var user_to string
        var message_string string
        var message_date string
        var message_time string
        var message_type int
        err := mr.Scan(&user_from, &user_to, &message_string, &message_date, &message_time, &message_type)
        checkError(err, "mr.Scan")
        msg := MessageStruct {
            UserFrom: user_from,
            UserTo: user_to,
            Content: message_string,
            Date: message_date,
            Time: message_time,
            Type: message_type,
        }
        messages = append(messages, msg)
    }
    // conn.WriteJSON(messages)
    enc, err:= json.Marshal(messages)
    checkError(err, "json.Marshal")
    conn.WriteMessage(websocket.TextMessage, enc)

    msgChan := make(chan Message)
    msgChans = append(msgChans, msgChan)
    println("msgChans is:", msgChans)

    var data_parsed [] interface { }

    go (func() {
        for {
            mt, message, err := conn.ReadMessage()
            if err != nil {
                fmt.Println("conn.ReadMessage error, now closing conn...")
                return
            }
            // checkError(err, "conn.ReadMessage")
            err = json.Unmarshal(message, &data_parsed)
            checkError(err, "json.Unmarshal")
            fmt.Println("data_parsed[0] is:", data_parsed[0])
            data_type, err := strconv.Atoi(data_parsed[0].(string))
            checkError(err, "strconv.Atoi")
            fmt.Println("data_type is:", data_type)
            switch data_type {
            case 0:
                user_from_cookie, err := r.Cookie("chatter_user_name")
                checkError(err, "[2] r.Cookie")
                user_from := user_from_cookie.Value
                user_to := data_parsed[1].(string)
                message_string := data_parsed[2].(string)
                date := data_parsed[3].(string)
                time := data_parsed[4].(string)
                message_type, err := strconv.Atoi(data_parsed[5].(string))
                checkError(err, "strconv.Atoi")
                if message_type == 0 {  // send to user
                    var user_id int
                    err := db.QueryRow("select user_id from users where user_name = ?", user_to).Scan(&user_id)
                    if err == sql.ErrNoRows {  // send user doesn't exist
                        errMsg, err := json.Marshal( []string { "9", user_to } )
                        checkError(err, "json.Marshal")
                        msgChan <- Message { websocket.TextMessage, errMsg }
                        continue
                    }
                } else {  // send to stream
                    var stream_id int
                    err := db.QueryRow("select stream_id from streams where stream_name = ?", user_to).Scan(&stream_id)
                    if err == sql.ErrNoRows {  // send stream doesn't exist
                        errMsg, err := json.Marshal( []string { "10", user_to } )
                        checkError(err, "json.Marshal")
                        msgChan <- Message { websocket.TextMessage, errMsg }
                        continue
                    }
                }
                checkError(err, "sql.Query")
                _, err = db.Exec("insert into messages(user_from, user_to, message, message_date, message_time, message_type) values(?, ?, ?, ?, ?, ?)",
                    user_from, user_to, message_string, date, time, message_type)
                checkError(err, "db.Exec")

                msg := MessageStruct {
                    UserFrom: user_from,
                    UserTo: user_to,
                    Content: message_string,
                    Date: date,
                    Time: time,
                    Type: message_type,
                }
                reply_data, err:= json.Marshal([]interface{} { "0", msg })
                checkError(err, "json.Marshal")
                // reply_data, err := json.Marshal([]string { "0", user_from, user_to, message_string, date, time, strconv.Itoa(message_type) })
                // checkError(err, "json.Marshal")
                for _, channel := range msgChans {
                    channel <- Message { mt: mt, message: reply_data }
                }
            case 1:  // create stream: [ "1", stream_name ]
                stream_name := data_parsed[1].(string)
                fmt.Println("stream_name is:", stream_name)

                _, err = db.Exec("insert into streams(stream_name) value(?)", stream_name)
                checkError(err, "db.Exec")

                reply_data, err := json.Marshal([]string { "1", stream_name })
                checkError(err, "json.Marshal")

                for _, channel := range msgChans {
                    channel <- Message { mt: mt, message: reply_data }
                }
            case 5: // [ "5", user_name, stream_name ] user subscribing to stream
                user_name := data_parsed[1].(string)
                stream_name := data_parsed[2].(string)

                fmt.Println("user_name is:", user_name)
                fmt.Println("stream_name is:", stream_name)

                var user_id uint64
                err := db.QueryRow("select user_id from users where user_name = ?", user_name).Scan(&user_id)
                checkError(err, "db.QueryRow(user_id)")

                var stream_id uint64
                err = db.QueryRow("select stream_id from streams where stream_name = ?", stream_name).Scan(&stream_id)
                checkError(err, "db.QueryRow(stream_id)")

                var unused int64
                err = db.QueryRow("select user_id from stream_subscribers where user_id = ? and stream_id = ?", user_id, stream_id).Scan(&unused)
                if err == sql.ErrNoRows {  // user hasn't subscribed
                    _, err = db.Exec("insert ignore into stream_subscribers(user_id, stream_id) values (?, ?)", user_id, stream_id)
                    checkError(err, "db.Exec(insert into stream_subscribers")
                    reply_data, err := json.Marshal([]string { "6", stream_name })
                    checkError(err, "json.Marshal")
                    msgChan <- Message { mt: mt, message: reply_data }
                } else {
                    checkError(err, "db.QueryRow")
                }
            case 7: // [ "7", stream_name ] asking for stream messages
                stream_name := data_parsed[1].(string)
                rows, err := db.Query(`select user_from, message, message_date, message_time, message_id
                    from messages where message_type = 1 and user_to = ?`, stream_name);
                checkError(err, "case 7")
                defer rows.Close()
                // reply := []map[string]interface{}{}
                reply := []interface{}{}
                reply = append(reply, []interface{}{"8", stream_name}...)
                for rows.Next() {
                    var user_from string
                    var message string
                    var message_date string
                    var message_time string
                    var message_id int64
                    err := rows.Scan(&user_from, &message, &message_date, &message_time, &message_id)
                    checkError(err, "case 7 row")
                    reply_entry := map[string]interface{}{}
                    reply_entry["user_from"] = user_from
                    reply_entry["message"] = message
                    reply_entry["message_date"] = message_date
                    reply_entry["message_time"] = message_time
                    reply_entry["message_id"] = message_id
                    reply = append(reply, reply_entry)
                }
                reply_json, err := json.Marshal(reply)
                checkError(err, "case 8 json")
                fmt.Println("XXX will send type 8")
                msgChan <- Message { mt: mt, message: reply_json }
            case 11:
                user_name := data_parsed[1].(string)
                stream_name := data_parsed[2].(string)

                fmt.Println("user_name is:", user_name)
                fmt.Println("stream_name is:", stream_name)

                var user_id uint64
                err := db.QueryRow("select user_id from users where user_name = ?", user_name).Scan(&user_id)
                checkError(err, "db.QueryRow(user_id)")

                var stream_id uint64
                err = db.QueryRow("select stream_id from streams where stream_name = ?", stream_name).Scan(&stream_id)
                checkError(err, "db.QueryRow(stream_id)")

                _, err = db.Exec("delete from stream_subscribers where (user_id, stream_id) = (?, ?)", user_id, stream_id)
                check(err)

                msgChan <- Message { mt: mt, message: message }
            }
        }
    })()

    for {
        message_struct := <- msgChan
        conn.WriteMessage(message_struct.mt, message_struct.message)
    }
}

func signupPage(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.ServeFile(w, r, "signup.html")
        return
    }

    username := r.FormValue("username")
    password := r.FormValue("password")

    fmt.Println("username is:", username)
    fmt.Println("password is:", password)

    var user string
    err := db.QueryRow("select user_name from users where user_name=?", username).Scan(&user)
    switch {
    case err == sql.ErrNoRows:
        _, err = db.Exec("insert into users(user_name, password) values(?, ?)", username, password)
        if err != nil {
            http.Error(w, "Error quering database.", 500)
            return
        }
        w.Write([]byte("User created."))
        return
    case err != nil:
        http.Error(w, "Error quering database.", 500)
        return
    default:
        http.Redirect(w, r, "/", 301)
    }
}

func loginPage(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.ServeFile(w, r, "login.html")
        return
    }

    username := r.FormValue("username")
    password := r.FormValue("password")

    fmt.Println("username is:", username)
    fmt.Println("password is:", password)

    var user string
    var pass string
    err := db.QueryRow("select user_name, password from users where user_name=?", username).Scan(&user, &pass)
    switch {
    case err == sql.ErrNoRows:
        http.Redirect(w, r, "/", 301)
        return
    case err != nil:
        http.Error(w, "Error quering database.", 500)
        return
    default:
        if password == pass {
            expires := time.Now().AddDate(0, 0, 1)
            cookie := http.Cookie {
                Name: "chatter_user_name",
                Expires: expires,
                Value: username,
            }
            http.SetCookie(w, &cookie)
        }
        http.Redirect(w, r, "/", 301)
    }
}

func logoutPage(w http.ResponseWriter, r *http.Request) {
    fmt.Println("in serveLogout, will log out")
    expired := time.Now().AddDate(0, 0, -1)
    cookie := http.Cookie {
        Name: "chatter_user_name",
        Expires: expired,
        Value: "",
    }
    http.SetCookie(w, &cookie)
    w.Write([]byte("BYE BYE"));
}

func main() {
    dataSourceName := ""
    db, err = sql.Open("mysql", dataSourceName)
    if err != nil {
        panic(err.Error())
    }
    defer db.Close()
    err = db.Ping()
    if err != nil {
        panic(err.Error())
    }

    fmt.Println("addr is:", ":8090")
    http.HandleFunc("/", serveHome)
    http.HandleFunc("/ws", serveWS)
    http.HandleFunc("/signup", signupPage)
    http.HandleFunc("/login", loginPage)
    http.HandleFunc("/logout", logoutPage)
    err = http.ListenAndServe(":8090", nil)
    // err = http.ListenAndServeTLS(":8090", "server.crt", "server.key", nil)
    checkError(err, "http.ListenAndServe")
}

func checkError(err error, msg string) {
    if err != nil {
        fmt.Printf("[%s] %s\n", msg, err)
        os.Exit(-1)
    }
}

func check(err error) {
    if err != nil {
        panic(err)
    }
}
