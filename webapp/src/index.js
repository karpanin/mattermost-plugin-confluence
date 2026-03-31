import manifest from './manifest';

import Hooks from './hooks';
import reducer from './reducers';
import {isSystemMessage} from 'mattermost-redux/utils/post_utils';
import {getPost} from 'mattermost-redux/selectors/entities/posts';

import SubscriptionModal from './components/subscription_modal';
import CreatePageModal from './components/create_page_modal';
import {openCreatePageModal, getSubscriptionAccess} from './actions';
import Constants from './constants';

//
// Define the plugin class that will register
// our plugin components.
//
class PluginClass {
    initialize(registry, store) {
        registry.registerReducer(reducer);
        registry.registerRootComponent(SubscriptionModal);
        registry.registerRootComponent(CreatePageModal);
        const hooks = new Hooks(store);
        registry.registerSlashCommandWillBePostedHook(hooks.slashCommandWillBePostedHook);
        registry.registerPostDropdownMenuAction({
            text: Constants.CREATE_PAGE_ACTION,
            action: async (postId) => {
                const state = store.getState();
                const post = getPost(state, postId);
                if (!post || isSystemMessage(post)) {
                    return;
                }

                const {data: subscriptionAccessData, error} = await getSubscriptionAccess()(store.dispatch);
                if (error || !subscriptionAccessData?.can_run_subscribe_command) {
                    return;
                }

                openCreatePageModal(postId)(store.dispatch);
            },
            filter: (postId) => {
                const state = store.getState();
                const post = getPost(state, postId);
                return Boolean(post && !isSystemMessage(post));
            },
        });
    }
}

//
// To register your plugin you must expose it on window.
//
window.registerPlugin(manifest.id, new PluginClass());
