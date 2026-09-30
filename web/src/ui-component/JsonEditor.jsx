import { lazy } from 'react';
import Loadable from './Loadable';

// Keep the channel list independent of the editor until a dialog actually mounts it.
const JsonEditor = Loadable(lazy(() => import('./JsonEditorRuntime')));

export default JsonEditor;
