from rag_pipe import get_rag_chain

if __name__ == "__main__":
    print("\n" + "=" * 60)
    print("RAG Demo - Example 1")
    print("=" * 60)

    rag_chain = get_rag_chain()
    answer = rag_chain.invoke("what is economy")
    print("========Answer========")
    print(answer)

    print("\n✅ Example completed!")
