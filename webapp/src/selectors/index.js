import manifest from '../manifest';

const getPluginState = (state) => state[`plugins-${manifest.id}`] || {};

const isSubscriptionModalVisible = (state) => getPluginState(state).subscriptionModal;

const isSubscriptionEditModalVisible = (state) => getPluginState(state).subscriptionEditModal;

const getCreatePageModal = (state) => getPluginState(state).createPageModal || {};
const getAddCommentModal = (state) => getPluginState(state).addCommentModal || {};
const getSubscriptionAccess = (state) => getPluginState(state).subscriptionAccess || {};

export default {
    isSubscriptionModalVisible,
    isSubscriptionEditModalVisible,
    getCreatePageModal,
    getAddCommentModal,
    getSubscriptionAccess,
};
