import os
from langchain_ollama import OllamaLLM
from langchain_core.runnables import RunnablePassthrough
from langchain_core.runnables import RunnableLambda
from langchain_core.output_parsers import StrOutputParser
from langchain_core.prompts import PromptTemplate
from embed import get_retriever


def format_docs(docs):
    """Format retrieved documents into a single string"""
    return "\n\n".join(doc.page_content for doc in docs)

def log_and_format_docs(docs):
    print("\n=== Retrieved Contexts ===")
    for i, doc in enumerate(docs, 1):
        print(doc.page_content[:300].replace("\n", " ") + "...\n")
    return format_docs(docs)

def get_llm():
    """Initialize and return Ollama LLM"""
    ollama_host = os.getenv('OLLAMA_HOST', 'http://axora-ollama:11434')
    ollama_model = os.getenv('OLLAMA_MODEL', 'phi3:mini')
    
    llm = OllamaLLM(
        base_url=ollama_host,
        model=ollama_model,
        temperature=0.7,
    )
    
    return llm


def get_rag_chain():
    llm = get_llm()
    retriever = get_retriever(search_type="similarity", search_kwargs={"k": 4})

    prompt = PromptTemplate.from_template(
        """
You are a helpful AI assistant. Use the following context to answer the question.

Context:
{context}

Question:
{input}

Answer clearly and concisely based only on the given context.
        """
    )

    rag_chain = (
        {
            "context": retriever | RunnableLambda(log_and_format_docs),
            "input": RunnablePassthrough(),
        }
        | prompt
        | llm
        | StrOutputParser()
    )

    return rag_chain