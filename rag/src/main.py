from rag_pipe import simple_rag_query

if __name__ == "__main__":
    print("\n" + "=" * 60)
    print("RAG Demo - Example 1")
    print("=" * 60)

    print("\n### Example 1: Simple Similarity Search")
    simple_rag_query(
        question="What is artificial intelligence?",
        search_type="similarity",
        k=3
    )

    print("\n✅ Example completed!")
