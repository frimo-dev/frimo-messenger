CREATE TABLE messages (
    message_id                      uuid,
    message_type                    text,
    created_at              timestamp,
    sender_id               uuid,

    topic_id                uuid,
    bucket                  date,

    current_version         int,
    text                    text,
    attachments_json        text,

    system_event_type       text,
    system_event_json       text,

    reply_to_message_id     uuid,

    deleted_for_everyone_at timestamp,
    deleted_for_everyone_by uuid,

    PRIMARY KEY ((topic_id, bucket), message_id)
) WITH CLUSTERING ORDER BY (message_id DESC);

CREATE TABLE message_versions (
    message_id       uuid,
    version          int,

    created_by       uuid,
    created_at       timestamp,

    text             text,
    attachments_json text,

    PRIMARY KEY (
        (message_id),
        version
    )
) WITH CLUSTERING ORDER BY (version DESC);

CREATE TABLE hidden_messages_by_user (
    user_id uuid,
    topic_id uuid,
    bucket date,
    message_id uuid,

    hidden_at timestamp,

    PRIMARY KEY (
        (user_id, topic_id, bucket),
        message_id
    )
);