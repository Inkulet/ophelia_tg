import { Routes } from '@angular/router';
import { HomeComponent } from './components/home/home.component';
import { AboutComponent } from './components/about/about.component';
import { SkillsComponent } from './components/skills/skills.component';
import { ProjectsComponent } from './components/projects/projects.component';
import { ContactComponent } from './components/contact/contact.component';
import { PostListComponent } from './components/post-list/post-list.component';
import { EventListComponent } from './components/event-list/event-list.component';

export const routes: Routes = [
  { path: '', component: HomeComponent },
  { path: 'news', component: PostListComponent },
  { path: 'events', component: EventListComponent },
  { path: 'about', component: AboutComponent },
  { path: 'skills', component: SkillsComponent },
  { path: 'projects', component: ProjectsComponent },
  { path: 'contact', component: ContactComponent },
];
