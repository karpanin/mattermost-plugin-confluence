import React from 'react';
import PropTypes from 'prop-types';
import {
    Button,
    Modal,
} from 'react-bootstrap';

import ConfluenceField from '../confluence_field';
import Validator from '../validator';

const initialState = {
    title: '',
    spaceKey: '',
    parentPageID: '',
    error: '',
    saving: false,
};

export default class CreatePageModal extends React.PureComponent {
    static propTypes = {
        modalState: PropTypes.object.isRequired,
        post: PropTypes.object,
        close: PropTypes.func.isRequired,
        createPageFromPost: PropTypes.func.isRequired,
    };

    static defaultProps = {
        post: null,
    };

    constructor(props) {
        super(props);
        this.state = initialState;
        this.validator = new Validator();
    }

    componentDidUpdate(prevProps) {
        if (this.props.modalState.postId && this.props.modalState.postId !== prevProps.modalState.postId) {
            this.setState({
                ...initialState,
                title: this.getDefaultTitle(),
            });
        }
    }

    getDefaultTitle = () => {
        const message = this.props.post?.message || '';
        const firstLine = message.split('\n').find((line) => line.trim());
        if (!firstLine) {
            return 'Mattermost note';
        }

        return firstLine.trim().slice(0, 80);
    };

    handleClose = (e) => {
        if (e && e.preventDefault) {
            e.preventDefault();
        }

        this.setState(initialState, this.props.close);
    };

    handleChange = (key) => (e) => {
        this.setState({[key]: e.target.value});
    };

    handleSubmit = async () => {
        if (!this.validator.validate()) {
            return;
        }

        this.setState({
            saving: true,
            error: '',
        });

        const response = await this.props.createPageFromPost({
            postID: this.props.modalState.postId,
            title: this.state.title.trim(),
            spaceKey: this.state.spaceKey.trim(),
            parentPageID: this.state.parentPageID.trim(),
        });

        if (response.error) {
            this.setState({
                saving: false,
                error: response.error.response?.text || 'Failed to create Confluence page.',
            });
            return;
        }

        this.handleClose();
    };

    render() {
        const visible = Boolean(this.props.modalState.postId);
        const {saving, error} = this.state;

        return (
            <Modal
                show={visible}
                onHide={this.handleClose}
                backdrop={'static'}
            >
                <Modal.Header closeButton={true}>
                    <Modal.Title>{'Create Confluence Page'}</Modal.Title>
                </Modal.Header>
                <Modal.Body>
                    <ConfluenceField
                        label={'Title'}
                        type={'text'}
                        fieldType={'input'}
                        required={true}
                        placeholder={'Enter the Confluence page title.'}
                        value={this.state.title}
                        addValidation={this.validator.addValidation}
                        removeValidation={this.validator.removeValidation}
                        onChange={this.handleChange('title')}
                    />
                    <ConfluenceField
                        label={'Space Key'}
                        type={'text'}
                        fieldType={'input'}
                        required={true}
                        placeholder={'Enter the Confluence Space Key.'}
                        value={this.state.spaceKey}
                        addValidation={this.validator.addValidation}
                        removeValidation={this.validator.removeValidation}
                        onChange={this.handleChange('spaceKey')}
                    />
                    <ConfluenceField
                        label={'Parent Page ID'}
                        type={'text'}
                        fieldType={'input'}
                        required={false}
                        placeholder={'Optional parent page ID.'}
                        value={this.state.parentPageID}
                        addValidation={this.validator.addValidation}
                        removeValidation={this.validator.removeValidation}
                        onChange={this.handleChange('parentPageID')}
                    />
                    {Boolean(error) && (
                        <p className='alert alert-danger'>
                            <i className='fa fa-warning' title='Warning Icon'/>
                            <span> {error}</span>
                        </p>
                    )}
                </Modal.Body>
                <Modal.Footer>
                    <Button
                        type='button'
                        bsStyle='link'
                        onClick={this.handleClose}
                    >
                        {'Cancel'}
                    </Button>
                    <Button
                        type='submit'
                        bsStyle='primary'
                        onClick={this.handleSubmit}
                        disabled={saving}
                    >
                        {saving && <span className='fa fa-spinner fa-fw fa-pulse spinner'/>}
                        {'Create Page'}
                    </Button>
                </Modal.Footer>
            </Modal>
        );
    }
}
