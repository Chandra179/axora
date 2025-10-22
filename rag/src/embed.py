import os
import requests
from typing import List
from qdrant_client import QdrantClient
from langchain_qdrant import QdrantVectorStore
from langchain_core.embeddings import Embeddings

class TEIEmbeddings(Embeddings):
    """Custom embeddings class for HuggingFace Text Embeddings Inference API"""
    
    def __init__(self, api_url: str):
        self.api_url = api_url
        
    def embed_documents(self, texts: List[str]) -> List[List[float]]:
        """Embed a list of documents"""
        response = requests.post(
            f"{self.api_url}/embed",
            json={"inputs": texts},
            timeout=30
        )
        response.raise_for_status()
        return response.json()
    
    def embed_query(self, text: str) -> List[float]:
        """Embed a single query"""
        response = requests.post(
            f"{self.api_url}/embed",
            json={"inputs": text},
            timeout=30
        )
        response.raise_for_status()
        return response.json()[0]

def get_vector_store():
    """Initialize and return Qdrant vector store with custom embeddings"""
    
    # Get configuration from environment
    qdrant_host = os.getenv('QDRANT_HOST', 'axora-qdrant')
    qdrant_port = int(os.getenv('QDRANT_PORT', 6333))
    collection_name = os.getenv('QDRANT_COLLECTION', 'crawl_collection')
    embedding_url = os.getenv('EMBEDDING_API_URL', 'http://axora-mpnetbasev2:8000')
    
    # Initialize Qdrant client
    client = QdrantClient(host=qdrant_host, port=qdrant_port)
    
    # Initialize custom embeddings
    embeddings = TEIEmbeddings(api_url=embedding_url)
    
    # Create vector store
    vector_store = QdrantVectorStore(
        client=client,
        collection_name=collection_name,
        embedding=embeddings
    )
    
    return vector_store


def get_retriever(search_type="similarity", search_kwargs=None):
    """
    Get a retriever with specified search strategy
    
    Args:
        search_type: Type of search to perform
            - "similarity": Standard similarity search (default)
            - "mmr": Maximal Marginal Relevance (for diversity)
            - "similarity_score_threshold": Filter by similarity score
        search_kwargs: Additional search parameters
            - k: Number of documents to retrieve (default: 4)
            - score_threshold: Minimum similarity score (for similarity_score_threshold)
            - fetch_k: Number of docs to fetch before MMR reranking (for mmr)
            - lambda_mult: Diversity factor 0-1, where 1 is max diversity (for mmr)
    """
    
    if search_kwargs is None:
        search_kwargs = {"k": 4}
    
    vector_store = get_vector_store()
    
    retriever = vector_store.as_retriever(
        search_type=search_type,
        search_kwargs=search_kwargs
    )
    
    return retriever