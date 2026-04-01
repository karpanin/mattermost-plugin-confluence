import manifest from '../manifest';

const {id} = manifest;

export const ACTION_TYPES = {
    OPEN_SUBSCRIPTION_MODAL: id + '_open_subscription_modal',
    CLOSE_SUBSCRIPTION_MODAL: id + '_close_subscription_modal',
    RECEIVED_SUBSCRIPTION: id + '_received_subscription',
    OPEN_CREATE_PAGE_MODAL: id + '_open_create_page_modal',
    CLOSE_CREATE_PAGE_MODAL: id + '_close_create_page_modal',
    OPEN_ADD_COMMENT_MODAL: id + '_open_add_comment_modal',
    CLOSE_ADD_COMMENT_MODAL: id + '_close_add_comment_modal',
    RECEIVED_SUBSCRIPTION_ACCESS: id + '_received_subscription_access',
};
