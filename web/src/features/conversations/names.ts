import type { ConversationEntry } from '@/infrastructure/api/types.gen'

/** What a Conversation is called: its title, else everyone in it. */
export const conversationName = (c: ConversationEntry) => c.title || c.members.map((m) => m.nickname).join(', ')
