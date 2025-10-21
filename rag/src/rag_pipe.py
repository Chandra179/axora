import os
from langchain_ollama import OllamaLLM
from langchain_core.prompts import ChatPromptTemplate
from langchain_core.output_parsers import StrOutputParser
from langchain_core.runnables import RunnablePassthrough
from load import get_retriever


def format_docs(docs):
    """Format retrieved documents into a single string"""
    return "\n\n".join(doc.page_content for doc in docs)


def get_llm():
    """Initialize and return Ollama LLM"""
    ollama_host = os.getenv('OLLAMA_HOST', 'http://axora-ollama:11434')
    ollama_model = os.getenv('OLLAMA_MODEL', 'mistral:7b-instruct-q4_0')
    
    llm = OllamaLLM(
        base_url=ollama_host,
        model=ollama_model,
        temperature=0.7,
    )
    
    return llm


def create_rag_chain(search_type="similarity", search_kwargs=None):
    """
    Create a RAG chain with specified retrieval strategy
    
    Args:
        search_type: Type of search ('similarity', 'mmr', 'similarity_score_threshold')
        search_kwargs: Additional search parameters (k, score_threshold, etc.)
    """
    
    # Get retriever
    retriever = get_retriever(search_type=search_type, search_kwargs=search_kwargs)
    
    # Get LLM
    llm = get_llm()
    
    # Create prompt template
    template = """You are a helpful assistant that answers questions based on the provided context. 
Use the following pieces of context to answer the question at the end. 
If you don't know the answer based on the context, just say that you don't know, don't try to make up an answer.

Context:
{context}

Question: {question}

Answer:"""
    
    prompt = ChatPromptTemplate.from_template(template)
    
    # Create RAG chain using LCEL (LangChain Expression Language)
    rag_chain = (
        {"context": retriever | format_docs, "question": RunnablePassthrough()}
        | prompt
        | llm
        | StrOutputParser()
    )
    
    return rag_chain


def simple_rag_query(question: str, search_type="similarity", k=4, verbose=True):
    """
    Simple RAG query function
    
    Args:
        question: User question
        search_type: Retrieval strategy
        k: Number of documents to retrieve
        verbose: Print retrieved documents
    """
    
    if verbose:
        print(f"\n{'='*60}")
        print(f"Question: {question}")
        print(f"Retrieval Strategy: {search_type}")
        print(f"{'='*60}\n")
    
    # Create RAG chain
    rag_chain = create_rag_chain(
        search_type=search_type,
        search_kwargs={"k": k}
    )
    
    # Get retriever for showing context (optional)
    if verbose:
        retriever = get_retriever(search_type=search_type, search_kwargs={"k": k})
        docs = retriever.invoke(question)
        
        print("Retrieved Context:")
        print("-" * 60)
        for i, doc in enumerate(docs, 1):
            print(f"\nDocument {i}:")
            print(doc.page_content[:300] + "...")
        print("\n" + "="*60 + "\n")
    
    # Generate answer
    answer = rag_chain.invoke(question)
    
    if verbose:
        print("Answer:")
        print("-" * 60)
        print(answer)
        print("\n" + "="*60 + "\n")
    
    return answer