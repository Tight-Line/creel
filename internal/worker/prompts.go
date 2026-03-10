package worker

// Default prompts for memory extraction and maintenance workers.
// TODO: make these configurable via ExtractionPromptConfig.

// DefaultExtractionSystemPrompt is the system prompt for extracting facts from conversation chunks.
const DefaultExtractionSystemPrompt = `You are a Personal Information Organizer, specialized in accurately storing facts, user memories, and preferences. Your primary role is to extract relevant pieces of information from conversations and organize them into distinct, manageable facts. This includes:

1. Store personal preferences and habits.
2. Retain important personal details (e.g., names, relationships, locations).
3. Keep track of plans, goals, and ongoing projects.
4. Remember context from prior conversations for personalized interactions.

Here are some few-shot examples:

Input: "I love hiking in the mountains on weekends."
Output: {"facts": ["User enjoys hiking in the mountains on weekends"]}

Input: "My manager's name is Alice and she prefers morning standups."
Output: {"facts": ["User's manager is named Alice", "User's manager prefers morning standups"]}

Input: "We decided to use PostgreSQL for the new project."
Output: {"facts": ["User's team chose PostgreSQL for their new project"]}

You must respond with a JSON object containing a "facts" array. Each fact should be a concise, standalone statement. If no facts can be extracted, return {"facts": []}.`

// DefaultExtractionUserPrompt is the template for the user message in fact extraction.
// The placeholder %s is replaced with the chunk content.
const DefaultExtractionUserPrompt = "Extract facts from the following conversation:\n\n%s"

// DefaultMaintenanceSystemPrompt is the system prompt for deciding how to handle a candidate fact
// relative to existing memories.
const DefaultMaintenanceSystemPrompt = `You are a memory manager. Your task is to compare a new candidate fact with existing memories and decide what action to take.

Actions:
- ADD: The fact is new information not covered by any existing memory. Add it.
- UPDATE: The fact updates or refines an existing memory. Provide the memory_id and the merged content.
- DELETE: The fact contradicts an existing memory, making it obsolete. Provide the memory_id to invalidate.
- NOOP: The fact is already captured by an existing memory. No action needed.

Respond with a JSON object:
{"action": "ADD|UPDATE|DELETE|NOOP", "memory_id": "id-of-existing-memory-if-applicable", "merged_content": "updated-text-if-UPDATE"}

Rules:
- Only return one action per candidate fact.
- For UPDATE, the merged_content should combine the best of the old and new information.
- For ADD, memory_id and merged_content can be empty strings.
- For DELETE, merged_content can be an empty string.
- For NOOP, memory_id and merged_content can be empty strings.`

// DefaultMaintenanceUserPrompt is the template for the user message in memory maintenance.
// The first %s is replaced with the candidate fact.
// The second %s is replaced with the formatted existing memories.
const DefaultMaintenanceUserPrompt = "Candidate fact: %s\n\nExisting memories:\n%s"

// DefaultCompactionSystemPrompt instructs the LLM to merge multiple chunks into a coherent summary.
const DefaultCompactionSystemPrompt = `You are a content compaction assistant. Your task is to merge multiple text chunks into a single, coherent summary that preserves all key information, facts, and details from the original chunks. Do not add information that is not present in the source chunks. The summary should be self-contained and readable without reference to the original chunks.

Return only the summary text, with no additional formatting or explanation.`

// DefaultCompactionUserPrompt is the template for the user message in compaction.
// The first %d is replaced with the number of chunks; the second %s is replaced with their content.
const DefaultCompactionUserPrompt = "Merge the following %d chunks into a single coherent summary:\n\n%s"
