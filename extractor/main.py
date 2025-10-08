from sub import KafkaClient

def handle_message(msg: bytes):
    print("Received:", msg.decode("utf-8"))

if __name__ == "__main__":
    client = KafkaClient.new_client("axora-kafka:9092")

    client.consume("test-topic", "test-group", handle_message)

    client.close()
