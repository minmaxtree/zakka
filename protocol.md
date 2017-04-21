1: on load server sends all streams names to each user,
   on user creating stream server sends newly created stream name to each user
    [ "1", stream_names... ]

4: server send messages to user

6: server sends to user subscribed stream names to client
    [ "6", sybscribed_stream_names... ]

7: client to server: ask for stream messages
    [ "7", stream_name ]

8: server to client: reply to stream messages request
    [ "8", stream_name messages... ]

9: error: user does not exist
    [ "9", user_name ]

10: error: stream does not exist
    [ "10", stream_name ]

11: client to server: unsubscribe stream
    server to client: acknowledge client unsubscribing stream
    [ "11", stream_name ]
