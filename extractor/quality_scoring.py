import structlog
import textstat
from langdetect import detect, LangDetectException
import nltk
from nltk.corpus import stopwords
from typing import Optional

logger = structlog.get_logger(__name__)

# Download stopwords if not already present
try:
    nltk.data.find('corpora/stopwords')
except LookupError:
    nltk.download('stopwords', quiet=True)


def calculate_quality_score(extracted_data: dict, target_language: str = "en") -> dict:
    """
    Calculate multi-factor quality score for extracted content.
    
    Args:
        extracted_data: Dictionary containing extracted content fields
        target_language: Expected language code (default: "en")
    
    Returns:
        Dictionary with 'passed' (bool), 'score' (float), and 'factors' (dict)
    """
    text = extracted_data.get("text", "")
    title = extracted_data.get("title")
    author = extracted_data.get("author")
    date = extracted_data.get("date")
    
    if not text:
        logger.warning("quality_score_no_text")
        return {"passed": False, "score": 0.0, "factors": {}}
    
    factors = {}
    
    # 1. Has Title
    factors["title"] = 1.0 if title else 0.0
    
    # 2. Text Length
    words = text.split()
    word_count = len(words)
    
    if word_count < 100:
        factors["length"] = 0.0
    elif word_count < 500:
        factors["length"] = 0.5
    elif word_count < 1500:
        factors["length"] = 0.8
    else:
        factors["length"] = 1.0
    
    # 3. Readability (Flesch Reading Ease)
    try:
        flesch_score = textstat.flesch_reading_ease(text)
        
        if flesch_score > 70:
            factors["readability"] = 1.0
        elif flesch_score >= 50:
            factors["readability"] = 0.8
        elif flesch_score >= 30:
            factors["readability"] = 0.5
        else:
            factors["readability"] = 0.2
    except Exception as e:
        logger.warning("readability_calculation_failed", error=str(e))
        factors["readability"] = 0.5  # Default to middle score
    
    # 4. Link Density
    links = extracted_data.get("links", [])
    links_count = len(links) if links else 0
    link_density = links_count / word_count if word_count > 0 else 0
    
    if link_density < 0.01:
        factors["link_density"] = 1.0
    elif link_density < 0.03:
        factors["link_density"] = 0.7
    elif link_density < 0.05:
        factors["link_density"] = 0.4
    else:
        factors["link_density"] = 0.2
    
    # 5. Language Detection
    try:
        detected_lang = detect(text)
        factors["language"] = 1.0 if detected_lang == target_language else 0.0
    except LangDetectException as e:
        logger.warning("language_detection_failed", error=str(e))
        factors["language"] = 0.5  # Default to middle score
    
    # 6. Stopword Ratio
    try:
        stop_words = set(stopwords.words('english'))
        stopword_count = sum(1 for word in words if word.lower() in stop_words)
        stop_ratio = stopword_count / word_count if word_count > 0 else 0
        
        if 0.35 <= stop_ratio <= 0.55:
            factors["stopword_ratio"] = 1.0
        elif (0.25 <= stop_ratio < 0.35) or (0.55 < stop_ratio <= 0.65):
            factors["stopword_ratio"] = 0.7
        else:
            factors["stopword_ratio"] = 0.3
    except Exception as e:
        logger.warning("stopword_calculation_failed", error=str(e))
        factors["stopword_ratio"] = 0.5  # Default to middle score
    
    # 7. Rich Metadata
    metadata_count = sum([
        1 if title else 0,
        1 if author else 0,
        1 if date else 0
    ])
    
    if metadata_count == 3:
        factors["metadata"] = 1.0
    elif metadata_count == 2:
        factors["metadata"] = 0.7
    elif metadata_count == 1:
        factors["metadata"] = 0.3
    else:
        factors["metadata"] = 0.0
    
    # Calculate Final Score (weighted sum)
    final_score = (
        factors["title"] * 0.10 +
        factors["length"] * 0.20 +
        factors["readability"] * 0.15 +
        factors["link_density"] * 0.10 +
        factors["language"] * 0.10 +
        factors["stopword_ratio"] * 0.15 +
        factors["metadata"] * 0.20
    )
    
    passed = final_score >= 0.7
    
    logger.info(
        "quality_score_calculated",
        score=round(final_score, 3),
        passed=passed,
        word_count=word_count,
        factors={k: round(v, 2) for k, v in factors.items()}
    )
    
    return {
        "passed": passed,
        "score": round(final_score, 3),
        "factors": factors
    }


def is_quality_content(extracted_data: dict, target_language: str = "en") -> bool:
    """
    Simple boolean check if content passes quality threshold.
    
    Args:
        extracted_data: Dictionary containing extracted content fields
        target_language: Expected language code (default: "en")
    
    Returns:
        Boolean indicating if content passed quality scoring (>= 0.7)
    """
    result = calculate_quality_score(extracted_data, target_language)
    return result["passed"]