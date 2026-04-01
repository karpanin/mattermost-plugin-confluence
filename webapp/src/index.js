import manifest from './manifest';

import Hooks from './hooks';
import reducer from './reducers';
import {isSystemMessage} from 'mattermost-redux/utils/post_utils';
import {getPost} from 'mattermost-redux/selectors/entities/posts';

import SubscriptionModal from './components/subscription_modal';
import CreatePageModal from './components/create_page_modal';
import AddCommentModal from './components/add_comment_modal';
import {openCreatePageModal, openAddCommentModal, getSubscriptionAccess} from './actions';
import Selectors from './selectors';
import CreatePageAction from './components/create_page_action';
import AddCommentAction from './components/add_comment_action';

//
// Define the plugin class that will register
// our plugin components.
//
class PluginClass {
    initialize(registry, store) {
        registry.registerReducer(reducer);
        registry.registerRootComponent(SubscriptionModal);
        registry.registerRootComponent(CreatePageModal);
        registry.registerRootComponent(AddCommentModal);
        const hooks = new Hooks(store);
        registry.registerSlashCommandWillBePostedHook(hooks.slashCommandWillBePostedHook);
        getSubscriptionAccess()(store.dispatch);
        registerConfluencePostAction(registry, store, CreatePageAction, openCreatePageModal);
        registerConfluencePostAction(registry, store, AddCommentAction, openAddCommentModal);
    }
}

function registerConfluencePostAction(registry, store, text, openModal) {
    registry.registerPostDropdownMenuAction({
        text,
        action: async (postId) => {
            const state = store.getState();
            const post = getPost(state, postId);
            if (!post || isSystemMessage(post)) {
                return;
            }

            const subscriptionAccessData = Selectors.getSubscriptionAccess(state);
            if (!subscriptionAccessData?.is_configured) {
                return;
            }

            if (subscriptionAccessData?.is_connected || subscriptionAccessData?.can_run_subscribe_command) {
                store.dispatch(openModal(postId));
                return;
            }

            window.open(`/plugins/${manifest.id}/api/v1/oauth2/connect`, '_blank');
        },
        filter: (postId) => {
            const state = store.getState();
            const post = getPost(state, postId);
            const subscriptionAccessData = Selectors.getSubscriptionAccess(state);
            return Boolean(post && !isSystemMessage(post) && subscriptionAccessData?.is_configured);
        },
    });
}

//
// To register your plugin you must expose it on window.
//
window.registerPlugin(manifest.id, new PluginClass());
