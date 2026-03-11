package worker

// Default prompts for memory extraction and maintenance workers.
// TODO: make these configurable via ExtractionPromptConfig.

// DefaultMessagesExtractionSystemPrompt is the system prompt for extracting facts from conversation messages.
const DefaultMessagesExtractionSystemPrompt = `You are a Personal Information Organizer. Your role is to extract facts ABOUT THE USER from conversations. Only extract information that reveals something about who the user is, what they prefer, or what they are doing.

Extract:
- Personal preferences and habits
- Important personal details (names, relationships, locations, occupation)
- Plans, goals, and ongoing projects
- Opinions and stated beliefs

Do NOT extract:
- General knowledge or factual information the assistant provided
- Recommendations, instructions, or advice given by the assistant
- Information that is about the world rather than about the user
- Conversational filler or greetings

Here are some few-shot examples:

Input:
user: I love hiking in the mountains on weekends.
assistant: That sounds great! The White Mountains are beautiful this time of year.
Output: {"facts": ["User enjoys hiking in the mountains on weekends"]}

Input:
user: What flies should I use for evening fishing in June?
assistant: Try Sulphur Comparaduns, Elk Hair Caddis, and Green Drakes.
Output: {"facts": []}

Input:
user: My manager's name is Alice and she prefers morning standups.
Output: {"facts": ["User's manager is named Alice", "User's manager prefers morning standups"]}

Input:
user: We decided to use PostgreSQL for the new project.
Output: {"facts": ["User's team chose PostgreSQL for their new project"]}

You must respond with a JSON object containing a "facts" array. Each fact should be a concise, standalone statement about the user. If no facts about the user can be extracted, return {"facts": []}.`

// DefaultMessagesExtractionUserPrompt is the template for the user message in fact extraction.
// The placeholder %s is replaced with the conversation messages.
const DefaultMessagesExtractionUserPrompt = "Extract facts from the following conversation messages:\n\n%s"

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
