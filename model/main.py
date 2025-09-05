import os
import requests
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from langchain_experimental.text_splitter import SemanticChunker
from typing import List
import logging
from langchain.docstore.document import Document

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

app = FastAPI()

EMBEDDING_SERVICE_URL = os.getenv("MPNETBASEV2_URL", "http://axora-mpnetbasev2:8000")

# Request model
class ChunkRequest(BaseModel):
    text: str

def get_embeddings(texts: List[str]) -> List[List[float]]:
    """Get embeddings from external service"""
    logging.info(f"Attempting to get embeddings for {len(texts)} texts from {EMBEDDING_SERVICE_URL}")
    try:
        logging.info(texts)
        response = requests.post(
            f"{EMBEDDING_SERVICE_URL}/embed",
            json={"inputs": texts},
            timeout=30
        )
        response.raise_for_status()
        embeddings = response.json()
        logging.info("Successfully received embeddings.")
        return embeddings
    except requests.exceptions.RequestException as e:
        logging.error(f"Request to embedding service failed: {e}")
        raise HTTPException(status_code=500, detail=f"Embedding service error: {str(e)}")
    except Exception as e:
        logging.error(f"An unexpected error occurred in get_embeddings: {e}")
        raise HTTPException(status_code=500, detail=f"An unexpected error occurred: {str(e)}")

# A custom wrapper for the external embedding service to integrate with LangChain
class ExternalEmbeddings:
    def embed_documents(self, texts: List[str]) -> List[List[float]]:
        return get_embeddings(texts)

embedding_model = ExternalEmbeddings()
text_splitter = SemanticChunker(embedding_model)

@app.post("/chunk")
async def chunk_text(request: ChunkRequest) -> List[List[float]]:
    """
    Chunk text semantically and return embeddings as array of arrays of floats
    """
    logging.info("Received a new text chunking request.")
    
    try:
        document = Document(page_content=request.text)
        
        logging.info(f"Splitting text of length {len(request.text)} using semantic chunking.")
        chunks = text_splitter.split_documents([document])
        
        if not chunks:
            logging.warning("No chunks were generated from the input text.")
            return []
        
        logging.info(f"Generated {len(chunks)} chunks.")
        logging.info(chunks)
        
        chunk_texts = [chunk.page_content for chunk in chunks]
        
        embeddings = get_embeddings(chunk_texts)
        
        logging.info("Successfully processed request and returning embeddings.")
        return embeddings
        
    except HTTPException as http_exc:
        raise http_exc
    except Exception as e:
        logging.error(f"An unexpected error occurred during text chunking: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/health")
async def health_check():
    logging.info("Health check endpoint was called.")
    return {"status": "ok"}

if __name__ == "__main__":
    logging.info("Starting Uvicorn server...")
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8001)