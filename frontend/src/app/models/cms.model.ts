export interface Post {
  id: string;
  title: string;
  content: string;
  mediaPath: string;
  createdAt: string;
  isHidden: boolean;
}

export interface Event {
  id: string;
  title: string;
  description: string;
  date: string;
  maxParticipants: number;
  currentParticipants: number[];
  mediaPath: string;
}
