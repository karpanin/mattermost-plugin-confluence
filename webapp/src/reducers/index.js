import {combineReducers} from 'redux';

import {subscriptionModal, subscriptionEditModal, createPageModal} from './subscription_modal';

export default combineReducers({
    subscriptionModal,
    subscriptionEditModal,
    createPageModal,
});
