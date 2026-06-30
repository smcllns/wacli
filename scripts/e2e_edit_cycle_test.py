import pathlib
import sys
import unittest

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))

from e2e_edit_cycle import build_plan, event_matches_text, response_data, normalize_target


class EditCycleTest(unittest.TestCase):
    def test_build_plan_names_first_and_subsequent_edit(self):
        plan = build_plan("ABC")
        self.assertEqual(plan.original_text, "WACLI_EDIT_E2E_ABC_original")
        self.assertEqual(plan.first_edit_text, "WACLI_EDIT_E2E_ABC_edit1")
        self.assertEqual(plan.second_edit_text, "WACLI_EDIT_E2E_ABC_edit2")

    def test_event_matches_exact_text_or_display_text_for_expected_chat(self):
        self.assertTrue(event_matches_text({"type": "message", "chatJid": "sender@s.whatsapp.net", "text": "hello"}, "hello", "sender@s.whatsapp.net"))
        self.assertTrue(event_matches_text({"type": "message", "chatJid": "sender@s.whatsapp.net", "displayText": "Edited hello → final"}, "final", "sender@s.whatsapp.net"))
        self.assertFalse(event_matches_text({"type": "message", "chatJid": "other@s.whatsapp.net", "text": "hello"}, "hello", "sender@s.whatsapp.net"))

    def test_response_data_requires_success_response_object(self):
        self.assertEqual(response_data(b'{"type":"response","success":true,"data":{"message_id":"m1"}}\n'), {"message_id": "m1"})
        with self.assertRaises(RuntimeError):
            response_data(b'{"type":"response","success":false,"error":"boom"}\n')

    def test_group_required_rejects_dm_send_chat(self):
        with self.assertRaisesRegex(ValueError, "group JID"):
            normalize_target("16504417792@s.whatsapp.net", None, True)

    def test_group_required_defaults_expect_chat_to_send_group(self):
        self.assertEqual(normalize_target("120363409936547663@g.us", None, True), "120363409936547663@g.us")

    def test_group_required_rejects_mismatched_expect_chat(self):
        with self.assertRaisesRegex(ValueError, "expect-chat"):
            normalize_target("120363409936547663@g.us", "16504417792@s.whatsapp.net", True)


if __name__ == "__main__":
    unittest.main()
