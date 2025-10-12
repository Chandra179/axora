import structlog
import textstat
from langdetect import detect, LangDetectException
from nltk.corpus import stopwords
import nltk

logger = structlog.get_logger(__name__)

# Download NLTK stopwords if not already present
try:
    nltk.data.find('corpora/stopwords')
except LookupError:
    nltk.download('stopwords', quiet=True)


def calculate_quality_score(extracted_data: dict, target_language: str = "en") -> dict:
    try:
        data = extracted_data.get("data", {})
        url = extracted_data.get("url", "")
        
        # Extract fields
        title = data.get("title", "")
        text = data.get("text", "")
        author = data.get("author", "")
        date = data.get("date", "")
        links = data.get("links", [])
        
        if not text:
            logger.warning("quality_check_no_text", url=url)
            return {"passed": False, "score": 0.0}
        
        # Initialize scores
        scores = {}
        
        # 1. Has Title (weight: 0.10)
        scores["title"] = 1.0 if title else 0.0
        
        # 2. Text Length (weight: 0.20)
        word_count = len(text.split())
        if word_count < 100:
            scores["length"] = 0.0
        elif word_count < 500:
            scores["length"] = 0.5
        elif word_count < 1500:
            scores["length"] = 0.8
        else:
            scores["length"] = 1.0
        
        # 3. Readability / Incoherent Detection (weight: 0.15)
        try:
            flesch_score = textstat.flesch_reading_ease(text)
            if flesch_score > 70:
                scores["readability"] = 1.0
            elif flesch_score >= 50:
                scores["readability"] = 0.8
            elif flesch_score >= 30:
                scores["readability"] = 0.5
            else:
                scores["readability"] = 0.2
        except Exception as e:
            logger.warning("readability_calculation_failed", url=url, error=str(e))
            scores["readability"] = 0.5  # neutral score on failure
        
        # 4. Link Density (weight: 0.10)
        link_count = len(links) if isinstance(links, list) else 0
        link_density = link_count / word_count if word_count > 0 else 0
        
        if link_density < 0.01:
            scores["link"] = 1.0
        elif link_density < 0.03:
            scores["link"] = 0.7
        elif link_density < 0.05:
            scores["link"] = 0.4
        else:
            scores["link"] = 0.2
        
        # 5. Language Detection (weight: 0.10)
        try:
            detected_lang = detect(text[:1000])  # Use first 1000 chars for efficiency
            scores["language"] = 1.0 if detected_lang == target_language else 0.0
        except LangDetectException as e:
            logger.warning("language_detection_failed", url=url, error=str(e))
            scores["language"] = 0.5  # neutral score on failure
        
        # 6. Stopword Ratio (weight: 0.15)
        try:
            stop_words = set(stopwords.words('english'))
            words = text.lower().split()
            stopword_count = sum(1 for word in words if word in stop_words)
            stop_ratio = stopword_count / len(words) if len(words) > 0 else 0
            
            if 0.35 <= stop_ratio <= 0.55:
                scores["stopword"] = 1.0
            elif (0.25 <= stop_ratio < 0.35) or (0.55 < stop_ratio <= 0.65):
                scores["stopword"] = 0.7
            else:
                scores["stopword"] = 0.3
        except Exception as e:
            logger.warning("stopword_calculation_failed", url=url, error=str(e))
            scores["stopword"] = 0.5  # neutral score on failure
        
        # 7. Rich Metadata (weight: 0.20)
        metadata_count = sum([
            bool(title),
            bool(author),
            bool(date)
        ])
        
        if metadata_count == 3:
            scores["metadata"] = 1.0
        elif metadata_count == 2:
            scores["metadata"] = 0.7
        elif metadata_count == 1:
            scores["metadata"] = 0.3
        else:
            scores["metadata"] = 0.0
        
        # Calculate final weighted score
        final_score = (
            scores["title"] * 0.10 +
            scores["length"] * 0.20 +
            scores["readability"] * 0.15 +
            scores["link"] * 0.10 +
            scores["language"] * 0.10 +
            scores["stopword"] * 0.15 +
            scores["metadata"] * 0.20
        )
        
        passed = final_score >= 0.7
        
        logger.info(
            "quality_score_calculated",
            url=url,
            final_score=round(final_score, 3),
            passed=passed,
            word_count=word_count,
            individual_scores={k: round(v, 2) for k, v in scores.items()}
        )
        
        return {
            "passed": passed,
            "score": round(final_score, 3),
            "details": scores
        }
        
    except Exception as e:
        logger.error(
            "quality_scoring_error",
            url=extracted_data.get("url", ""),
            error=str(e),
            exc_info=True
        )
        return {"passed": False, "score": 0.0}


def passes_quality_check(extracted_data: dict, target_language: str = "en") -> bool:
    """
    Simple boolean check if content passes quality threshold.
    
    Args:
        extracted_data: Dictionary containing 'url' and 'data' keys
        target_language: Expected language code (default: "en")
    
    Returns:
        True if quality score >= 0.7, False otherwise
    """
    result = calculate_quality_score(extracted_data, target_language)
    return result["passed"]