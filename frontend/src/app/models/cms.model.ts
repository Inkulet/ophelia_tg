export interface SiteSettings {
  id: string;
  backgroundURL: string;
  avatarURL: string;
  homeDescription: string;
  aboutText: string;
  contactEmail: string;
  contactPhone: string;
  contactLocation: string;
}

export interface Event {
  id: string;
  title: string;
  description: string;
  date: string;
  time: string;
  location: string;
  maxParticipants: number;
  currentParticipants: number[];
}

export interface Project {
  id: string;
  title: string;
  shortDescription: string;
  detailedContent: string;
  mediaURL: string;
}

export interface Woman {
  id: number;
  name: string;
  biography: string;
  photoURL: string;
  century: string;
  spheres: string[];
}

export interface WomenPage {
  items: Woman[];
  limit: number;
  offset: number;
  total: number;
}

export interface Post {
  id: string;
  title: string;
  content: string;
  mediaPath: string;
  createdAt: string;
  isHidden: boolean;
}
