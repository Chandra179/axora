from abc import ABC, abstractmethod
from confluent_kafka import Producer, Consumer


class CrawlEvent(ABC):
    """Abstract interface for Kafka event clients."""
    @abstractmethod
    def consume(self, topic: str, group_id: str, on_message) -> None:
        pass

    @abstractmethod
    def close(self) -> None:
        pass


class KafkaClient(CrawlEvent):
    """Concrete Kafka client implementation."""

    def __init__(self, url: str):
        self.url = url
        self.producer = Producer({"bootstrap.servers": url})
        self.consumer = None

    @staticmethod
    def new_client(url: str) -> "KafkaClient":
        """Factory method to create a new Kafka client."""
        return KafkaClient(url)

    def consume(self, topic: str, group_id: str, on_message) -> None:
        """Consume messages from a Kafka topic."""
        self.consumer = Consumer({
            "bootstrap.servers": self.url,
            "group.id": group_id,
            "auto.offset.reset": "earliest"
        })

        self.consumer.subscribe([topic])

        try:
            while True:
                msg = self.consumer.poll(1.0)
                if msg is None:
                    continue
                if msg.error():
                    print(f"Consumer error: {msg.error()}")
                    continue
                # call the callback
                on_message(msg.value())
        except KeyboardInterrupt:
            pass
        finally:
            self.consumer.close()

    def close(self) -> None:
        """Close Kafka producer/consumer."""
        if self.consumer:
            self.consumer.close()
        self.producer.flush()