import manifest from '../manifest';

const getPluginState = (state) => state[`plugins-${manifest.id}`] || {};

const isSubscriptionModalVisible = (state) => getPluginState(state).subscriptionModal;

const isSubscriptionEditModalVisible = (state) => getPluginState(state).subscriptionEditModal;

const getCreatePageModal = (state) => getPluginState(state).createPageModal || {};

export default {
    isSubscriptionModalVisible,
    isSubscriptionEditModalVisible,
    getCreatePageModal,
};
