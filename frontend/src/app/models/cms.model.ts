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

export interface NewsPost {
  id: string;
  text: string;
  imageURL: string;
  postURL: string;
}

export interface Post {
  id: string;
  title: string;
  content: string;
  mediaPath: string;
  createdAt: string;
  isHidden: boolean;
}
