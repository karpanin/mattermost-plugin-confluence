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
        registry.registerPostDropdownMenuAction({
            text: CreatePageAction,
            action: (postId) => {
                const state = store.getState();
                const post = getPost(state, postId);
                if (!post || isSystemMessage(post)) {
                    return;
                }

                const subscriptionAccessData = Selectors.getSubscriptionAccess(state);
                if (subscriptionAccessData?.is_configured === false) {
                    return;
                }

                if (subscriptionAccessData?.is_connected === false && !subscriptionAccessData?.can_run_subscribe_command) {
                    window.open(`/plugins/${manifest.id}/api/v1/oauth2/connect`, '_blank');
                    return;
                }

                if (subscriptionAccessData?.is_connected || subscriptionAccessData?.can_run_subscribe_command || Object.keys(subscriptionAccessData || {}).length === 0) {
                    store.dispatch(openCreatePageModal(postId));
                    return;
                }
            },
            filter: (postId) => {
                const state = store.getState();
                const post = getPost(state, postId);
                const subscriptionAccessData = Selectors.getSubscriptionAccess(state);
                return Boolean(post && !isSystemMessage(post) && subscriptionAccessData?.is_configured !== false);
            },
        });

        registry.registerPostDropdownMenuAction({
            text: AddCommentAction,
            action: (postId) => {
                const state = store.getState();
                const post = getPost(state, postId);
                if (!post || isSystemMessage(post)) {
                    return;
                }

                const subscriptionAccessData = Selectors.getSubscriptionAccess(state);
                if (subscriptionAccessData?.is_configured === false) {
                    return;
                }

                if (subscriptionAccessData?.is_connected === false && !subscriptionAccessData?.can_run_subscribe_command) {
                    window.open(`/plugins/${manifest.id}/api/v1/oauth2/connect`, '_blank');
                    return;
                }

                if (subscriptionAccessData?.is_connected || subscriptionAccessData?.can_run_subscribe_command || Object.keys(subscriptionAccessData || {}).length === 0) {
                    store.dispatch(openAddCommentModal(postId));
                    return;
                }
            },
            filter: (postId) => {
                const state = store.getState();
                const post = getPost(state, postId);
                const subscriptionAccessData = Selectors.getSubscriptionAccess(state);
                return Boolean(post && !isSystemMessage(post) && subscriptionAccessData?.is_configured !== false);
            },
        });
    }
}

//
// To register your plugin you must expose it on window.
//
window.registerPlugin(manifest.id, new PluginClass());
