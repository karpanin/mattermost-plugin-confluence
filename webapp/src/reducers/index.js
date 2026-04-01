import {combineReducers} from 'redux';

import {subscriptionModal, subscriptionEditModal, createPageModal, addCommentModal, subscriptionAccess} from './subscription_modal';

export default combineReducers({
    subscriptionModal,
    subscriptionEditModal,
    createPageModal,
    addCommentModal,
    subscriptionAccess,
});
