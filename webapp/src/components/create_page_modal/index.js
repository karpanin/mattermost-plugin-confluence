import {connect} from 'react-redux';
import {bindActionCreators} from 'redux';
import {getPost} from 'mattermost-redux/selectors/entities/posts';

import {closeCreatePageModal, createPageFromPost} from '../../actions';
import Selectors from '../../selectors';

import CreatePageModal from './create_page_modal';

const mapStateToProps = (state) => {
    const modalState = Selectors.getCreatePageModal(state);

    return {
        modalState,
        post: modalState.postId ? getPost(state, modalState.postId) : null,
    };
};

const mapDispatchToProps = (dispatch) => bindActionCreators({
    close: closeCreatePageModal,
    createPageFromPost,
}, dispatch);

export default connect(mapStateToProps, mapDispatchToProps)(CreatePageModal);
