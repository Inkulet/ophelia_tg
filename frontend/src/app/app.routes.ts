import { Routes } from '@angular/router';
import { HomeComponent } from './components/home/home.component';
import { AboutComponent } from './components/about/about.component';
import { ProjectsComponent } from './components/projects/projects.component';
import { ContactComponent } from './components/contact/contact.component';
import { EventListComponent } from './components/event-list/event-list.component';
import { WomanArchiveComponent } from './components/woman-archive/woman-archive.component';

export const routes: Routes = [
  { path: '', component: HomeComponent },
  { path: 'archive', component: WomanArchiveComponent },
  { path: 'news', redirectTo: 'archive', pathMatch: 'full' },
  { path: 'events', component: EventListComponent },
  { path: 'about', component: AboutComponent },
  { path: 'projects', component: ProjectsComponent },
  { path: 'contact', component: ContactComponent },
];
