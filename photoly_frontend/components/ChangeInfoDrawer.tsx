import React, {ChangeEvent, useState} from "react";
import {
    Drawer,
    DrawerBody,
    DrawerFooter,
    DrawerHeader,
    DrawerOverlay,
    DrawerContent,
    DrawerCloseButton,
    Input,
    Button, Text, useBoolean, InputRightElement, InputGroup
} from '@chakra-ui/react'
import useApi from "../hooks/useApi";
import useToken from "../hooks/useToken";
import Router from "next/router"

interface ChangeInfoProps {
    isOpen: boolean,
    onClose: () => void
}

const ChangeInfoDrawer: React.FC<ChangeInfoProps> = ({isOpen, onClose}) => {
    const [file, setFile] = useState<File>()
    const [username, setUsername] = useState<string>()
    const [email, setEmail] = useState<string>()
    const [oldPwd, setOldPwd] = useState<string>();
    const [newPwd, setNewPwd] = useState<string>();
    const token = useToken()
    const {post, isLoading} = useApi()
    const [show, setShow] = useBoolean(false)

    const handleFileChange = (e: ChangeEvent<HTMLInputElement>) => {
        if (!!e.target.files) setFile(e.target.files[0])
    }
    const handleUsernameChange = (e: ChangeEvent<HTMLInputElement>) => {
        setUsername(e.target.value)
    }
    const handleEmailChange = (e: ChangeEvent<HTMLInputElement>) => {
        setEmail(e.target.value)
    }
    const handleOldPwdChange = (e: ChangeEvent<HTMLInputElement>) => {
        setOldPwd(e.target.value)
    }
    const handleNewPwdChange = (e: ChangeEvent<HTMLInputElement>) => {
        setNewPwd(e.target.value)
    }
    const handleSubmit = async () => {
        let requested = false
        let success = true
        if (file !== undefined){
            requested = true
            await post("/user/updateAvatar", {
                file: file
            }, {
                headers: {
                    "HRD-token": token,
                    "Content-type": "multipart/form-data"
                }
            }).then(res=>{success &&= (!!res && res.data.msgCode === 200)})
        }
        if (username !== undefined) {
            requested = true
            await post(`/user/updateUsername?username=${username}`, {}, {
                headers: {
                    "HRD-token": token
                }
            }).then(res=>{success &&= (!!res && res.data.msgCode === 200)})
        }
        if (email !== undefined) {
            requested = true
            await post(`/user/updateEmail?email=${email}`, {}, {
                headers: {
                    "HRD-token": token
                }
            }).then(res=>{success &&= (!!res && res.data.msgCode === 200)})
        }
        if (oldPwd !== undefined && newPwd !== undefined) {
            requested = true
            await post(`/user/updatePassword?oldPass=${oldPwd}&newPass=${newPwd}`, {}, {
                headers:{
                    "HRD-token": token
                }
            }).then(res=>{success &&= (!!res && res.data.msgCode === 200)})
        }
        if (success && requested) Router.reload()
    }

    return (
        <Drawer
            isOpen={isOpen}
            placement='right'
            onClose={onClose}
        >
            <DrawerOverlay/>
            <DrawerContent>
                <DrawerCloseButton/>
                <DrawerHeader>Change Info</DrawerHeader>
                <DrawerBody>
                    <Input placeholder='Username' onChange={handleUsernameChange} value={username}/>
                    <Input placeholder='Email' mt={4} onChange={handleEmailChange} value={email}/>
                    <Text fontWeight={"semibold"} mt={4} fontSize={"xl"}>Change Avatar</Text>
                    <Input type={"file"} mt={4} accept={"image/png, image/jpeg"} onChange={handleFileChange}/>
                    <Text fontWeight={"semibold"} mt={4} fontSize={"xl"}>Change Password</Text>
                    <Input placeholder='Old Password' type={"password"} mt={4} onChange={handleOldPwdChange} value={oldPwd}/>
                    <InputGroup mt={4}>
                        <Input variant="outline" type={show ? 'text' : 'password'} placeholder="Enter password"
                               onChange={handleNewPwdChange} isDisabled={isLoading}></Input>
                        <InputRightElement pr={1}>
                            <Button padding={4} size='sm' onClick={setShow.toggle} fontSize="xs">
                                {show ? 'Hide' : 'Show'}
                            </Button>
                        </InputRightElement>
                    </InputGroup>
                </DrawerBody>

                <DrawerFooter>
                    <Button variant='outline' mr={3} onClick={onClose}>
                        Cancel
                    </Button>
                    <Button colorScheme='blue' onClick={() => {
                        handleSubmit();
                        onClose()
                    }} isLoading={isLoading} loadingText={"Saving..."}>Save</Button>
                </DrawerFooter>
            </DrawerContent>
        </Drawer>
    )
}

export default ChangeInfoDrawer