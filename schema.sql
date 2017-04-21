create table if not exists users(
    user_id int(64) not null auto_increment primary key,
    user_name varchar(64),
    password varchar(128)
);

create table if not exists messages(
    message_id int(64) not null auto_increment primary key,
    user_from varchar(32),
    user_to varchar(32),
    message text,
    message_date date,
    message_time time
    message_type int(8) -- 0: to user, 1: to stream
);

create table if not exists streams(
    stream_id int(32) not null auto_increment primary key,
    stream_name varchar(64)
);

create table if not exists stream_subscribers(
    user_id int(64),
    stream_id int(32),
    foreign key (stream_id) references streams (stream_id)
        on delete cascade,
    unique key subscription (user_id, stream_id)
);

-- create table if not exists stream_messages(
--     message_id int(64),
--     stream_id int(32),
--     foreign key (stream_id) references streams (stream_id)
--         on delete cascade,
--     unique key message_stream_comb (message_id, stream_id)
-- );
