import { create } from 'zustand';

interface ChatState {
  currentSessionId: string | null;
  messages: Array<{
    id: string;
    content: string;
    role: 'user' | 'assistant';
    timestamp: number;
    model?: string;
  }>;
  isStreaming: boolean;
  selectedModel: string | null;
  setCurrentSessionId: (id: string | null) => void;
  addMessage: (message: Omit<ChatState['messages'][number], 'id'> & { id?: string }) => void;
  clearMessages: () => void;
  setIsStreaming: (streaming: boolean) => void;
  setSelectedModel: (model: string | null) => void;
}

export const useChatStore = create<ChatState>((set) => ({
  currentSessionId: null,
  messages: [],
  isStreaming: false,
  selectedModel: null,
  setCurrentSessionId: (id) => set({ currentSessionId: id }),
  addMessage: (message) =>
    set((state) => ({
      messages: [
        ...state.messages,
        {
          ...message,
          id: message.id || Math.random().toString(36).substr(2, 9),
        },
      ],
    })),
  clearMessages: () => set({ messages: [] }),
  setIsStreaming: (streaming) => set({ isStreaming: streaming }),
  setSelectedModel: (model) => set({ selectedModel: model }),
}));