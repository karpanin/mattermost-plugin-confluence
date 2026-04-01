import {PostTypes} from 'mattermost-redux/action_types';

import Client from '../client';
import Constants from '../constants';

export const saveChannelSubscription = (body) => {
    return async () => {
        let data = null;
        try {
            data = await Client.saveChannelSubscription(body);
        } catch (error) {
            return {
                data,
                error,
            };
        }

        return {
            data,
            error: null,
        };
    };
};

export const editChannelSubscription = (body) => {
    return async () => {
        let data = null;
        try {
            data = await Client.editChannelSubscription(body);
        } catch (error) {
            return {
                data,
                error,
            };
        }

        return {
            data,
            error: null,
        };
    };
};

export function getSubscriptionAccess() {
    return async (dispatch) => {
        let data = null;
        let error = null;

        try {
            data = await Client.getSubscriptionAccess();
            if (dispatch) {
                dispatch({
                    type: Constants.ACTION_TYPES.RECEIVED_SUBSCRIPTION_ACCESS,
                    data,
                });
            }
        } catch (e) {
            error = e;
        }

        return {data, error};
    };
}

export function getPluginConfig() {
    return async () => {
        let data = null;
        let error = null;

        try {
            data = await Client.getPluginConfig();
        } catch (e) {
            error = e;
        }

        return {data, error};
    };
}

export const openSubscriptionModal = () => (dispatch) => {
    dispatch({
        type: Constants.ACTION_TYPES.OPEN_SUBSCRIPTION_MODAL,
    });
};

export const closeSubscriptionModal = () => (dispatch) => {
    dispatch({
        type: Constants.ACTION_TYPES.CLOSE_SUBSCRIPTION_MODAL,
    });
};

export const openCreatePageModal = (postId) => (dispatch) => {
    dispatch({
        type: Constants.ACTION_TYPES.OPEN_CREATE_PAGE_MODAL,
        data: {postId},
    });
};

export const closeCreatePageModal = () => (dispatch) => {
    dispatch({
        type: Constants.ACTION_TYPES.CLOSE_CREATE_PAGE_MODAL,
    });
};

export const openAddCommentModal = (postId) => (dispatch) => {
    dispatch({
        type: Constants.ACTION_TYPES.OPEN_ADD_COMMENT_MODAL,
        data: {postId},
    });
};

export const closeAddCommentModal = () => (dispatch) => {
    dispatch({
        type: Constants.ACTION_TYPES.CLOSE_ADD_COMMENT_MODAL,
    });
};

export const createPageFromPost = (body) => {
    return async () => {
        let data = null;
        try {
            data = await Client.createPageFromPost(body);
        } catch (error) {
            return {
                data,
                error,
            };
        }

        return {
            data,
            error: null,
        };
    };
};

export const addCommentToPageFromPost = (body) => {
    return async () => {
        let data = null;
        try {
            data = await Client.addCommentToPageFromPost(body);
        } catch (error) {
            return {
                data,
                error,
            };
        }

        return {
            data,
            error: null,
        };
    };
};

export const getCreatePageSpaces = () => {
    return async () => {
        let data = null;
        try {
            data = await Client.getCreatePageSpaces();
        } catch (error) {
            return {
                data,
                error,
            };
        }

        return {
            data,
            error: null,
        };
    };
};

export const searchCreatePageParents = (spaceKey, query) => {
    return async () => {
        let data = null;
        try {
            data = await Client.searchCreatePageParents(spaceKey, query);
        } catch (error) {
            return {
                data,
                error,
            };
        }

        return {
            data,
            error: null,
        };
    };
};

export const getChannelSubscription = (channelID, alias, userID) => async (dispatch) => {
    try {
        const response = await Client.getChannelSubscription(channelID, alias);
        dispatch({
            type: Constants.ACTION_TYPES.RECEIVED_SUBSCRIPTION,
            data: response,
        });
    } catch (e) {
        dispatch(sendEphemeralPost(e.response.text, channelID, userID));
    }
};

export function sendEphemeralPost(message, channelID, userID) {
    const timestamp = Date.now();
    const post = {
        id: 'confluencePlugin' + timestamp,
        user_id: userID,
        channel_id: channelID,
        message,
        type: 'system_ephemeral',
        create_at: timestamp,
        update_at: timestamp,
        root_id: '',
        parent_id: '',
        props: {},
    };

    return {
        type: PostTypes.RECEIVED_NEW_POST,
        data: post,
        channelID,
    };
}
