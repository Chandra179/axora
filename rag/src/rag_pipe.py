import os
from langchain_ollama import OllamaLLM
from langchain_core.runnables import RunnablePassthrough
from langchain_core.runnables import RunnableLambda
from langchain_core.output_parsers import StrOutputParser
from langchain_core.prompts import PromptTemplate
from embed import get_retriever, get_vector_store


def log(docs):
    print("\n" + "=" * 60)
    print("RETRIEVED CONTEXTS")
    print("=" * 60)
    print(docs)


def get_llm():
    """Initialize and return Ollama LLM"""
    ollama_host = os.getenv('OLLAMA_HOST', 'http://axora-ollama:11434')
    ollama_model = os.getenv('OLLAMA_MODEL', 'phi3:mini')
    
    print(f"Initializing LLM:")
    print(f"  - Model: {ollama_model}")
    print(f"  - Host: {ollama_host}\n")
    
    llm = OllamaLLM(
        base_url=ollama_host,
        model=ollama_model,
        temperature=0.7,
    )
    
    return llm


def get_rag_chain():    
    llm = get_llm()
    retriever = get_retriever(
        search_type="similarity", 
        search_kwargs={"k": 4}
    )

    prompt = PromptTemplate.from_template(
        """You are a helpful AI assistant. Use the following context to answer the question.

Context:
{context}

Question:
{input}

Answer clearly and concisely based only on the given context. If the context doesn't contain relevant information, say so.
"""
    )
    
    def log_docs(docs):
        print("\n" + "=" * 60)
        print("RETRIEVED CONTEXTS")
        print("=" * 60)
        for i, doc in enumerate(docs, 1):
            print(f"[{i}] {doc.metadata.get('source', 'unknown')}")
            print(doc.page_content[:300].replace("\n", " ") + "...\n")
        # Return the formatted string for downstream
        return "\n\n".join(doc.page_content for doc in docs)

    def log_prompt(inputs):
        print("\n" + "=" * 60)
        print("FINAL PROMPT SENT TO LLM")
        print("=" * 60)
        # inputs here has 'context' and 'input' keys already populated
        rendered = prompt.format(**inputs)
        print(rendered)
        return rendered

    rag_chain = (
        {
            "context": retriever | RunnableLambda(log_docs),
            "input": RunnablePassthrough(),
        }
        | RunnableLambda(log_prompt)
        | llm
        | StrOutputParser()
    )

    return rag_chain